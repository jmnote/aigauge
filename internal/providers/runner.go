package providers

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// maxCommandOutput caps how much of a CLI's stdout or stderr is kept in memory.
// The status commands print a line or two; anything past this is a runaway
// process or a wrapped HTTP body, neither of which we parse.
const maxCommandOutput = 64 * 1024

// commandResult is one finished CLI invocation. ExitCode is data, not failure:
// every provider's state mapping keys off the exit code, so a non-zero exit is
// returned with a nil error and only a process that could not be started or was
// killed produces an error.
type commandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// commandRunner is the seam that keeps provider diagnosis testable. Production
// code uses execRunner; tests inject a stub that replays recorded exit codes
// and output, so the suite never depends on Claude, Codex, or agy being
// installed, signed in, or reachable on the machine running it.
type commandRunner interface {
	run(ctx context.Context, name string, args ...string) (commandResult, error)
}

// pathLookup resolves an executable name to a full path, mirroring
// exec.LookPath. It is injected alongside commandRunner so a test can describe
// an installed or a missing CLI without touching the real PATH.
type pathLookup func(name string) (string, error)

// execRunner runs the real executable. It never goes through a shell - the
// executable and its arguments stay separate - and it does not create a console
// window on Windows.
type execRunner struct{}

func (execRunner) run(ctx context.Context, name string, args ...string) (commandResult, error) {
	command := exec.CommandContext(ctx, name, args...)
	configureHiddenCommand(command)

	stdout := &limitedBuffer{limit: maxCommandOutput}
	stderr := &limitedBuffer{limit: maxCommandOutput}
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()
	result := commandResult{Stdout: stdout.String(), Stderr: stderr.String()}

	// Check the context before looking at err. When a CommandContext deadline
	// expires the child is killed, and on Windows that surfaces as an ordinary
	// *exec.ExitError with exit code 1 - indistinguishable from a CLI reporting
	// "not signed in" unless we ask the context first. Getting this backwards
	// would tell a signed-in user on a slow network that they are signed out,
	// which is exactly the misclassification the state mapping forbids.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			result.ExitCode = exitError.ExitCode()
			return result, nil
		}
		return result, err
	}
	return result, nil
}

// limitedBuffer keeps at most limit bytes and silently drops the rest. Write
// always reports the full length as written so an oversized output never turns
// into a write error that would kill the child process mid-run - we want
// whatever the CLI said first, not a broken pipe.
type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if remaining := b.limit - b.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string { return b.buf.String() }
