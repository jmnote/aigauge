package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type callbackResult struct {
	code string
	err  error
}

// LoopbackServer manages the temporary HTTP listener on localhost for capturing OAuth redirects.
type LoopbackServer struct {
	listener net.Listener
	server   *http.Server
	host     string
	port     int
	path     string
	expected string
	resultCh chan callbackResult
	once     sync.Once
}

// StartLoopbackServer starts an HTTP listener bound to 127.0.0.1 and waits
// for the callback with the given state. port and path fix the listener's
// address and callback route; pass port 0 for "any free port" and path ""
// for the original "/oauth/callback" default. A provider whose OAuth client
// only has one exact redirect_uri registered (every provider config here
// does, since none of them are a client AI Gauge registered itself) needs
// both fixed to match that registration exactly. host controls only the
// hostname CallbackURL reports back as redirect_uri - the listener itself
// always binds the loopback IP - since some providers' OAuth clients have
// that redirect_uri registered as "localhost" rather than "127.0.0.1" and
// reject a technically-equivalent but textually different one; pass "" for
// the "127.0.0.1" default.
func StartLoopbackServer(expectedState string, port int, path string, host string) (*LoopbackServer, error) {
	if path == "" {
		path = "/oauth/callback"
	}
	if host == "" {
		host = "127.0.0.1"
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		if port != 0 {
			return nil, fmt.Errorf("failed to bind loopback listener on port %d (required by this provider's OAuth client - is another program already using it?): %w", port, err)
		}
		return nil, fmt.Errorf("failed to bind loopback listener: %w", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return nil, fmt.Errorf("listener address is not TCP: %v", listener.Addr())
	}

	ls := &LoopbackServer{
		listener: listener,
		host:     host,
		port:     tcpAddr.Port,
		path:     path,
		expected: expectedState,
		resultCh: make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, ls.handleCallback)

	ls.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := ls.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			ls.sendResult("", fmt.Errorf("loopback server error: %w", err))
		}
	}()

	return ls, nil
}

// CallbackURL returns the full callback redirect URI for this loopback listener.
func (ls *LoopbackServer) CallbackURL() string {
	return fmt.Sprintf("http://%s:%d%s", ls.host, ls.port, ls.path)
}

func (ls *LoopbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	state := query.Get("state")
	errParam := query.Get("error")
	code := query.Get("code")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if state == "" || state != ls.expected {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>AI Gauge - Authentication Failed</title><style>body{font-family:system-ui,sans-serif;text-align:center;padding:50px;background:#1e1e1e;color:#fff;}h1{color:#ff6b6b;}</style></head><body><h1>Authentication Failed</h1><p>Invalid or expired state parameter. Please return to AI Gauge and try again.</p></body></html>`))
		ls.sendResult("", fmt.Errorf("state mismatch: expected %q, got %q", ls.expected, state))
		return
	}

	if errParam != "" {
		desc := query.Get("error_description")
		if desc == "" {
			desc = errParam
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html><head><title>AI Gauge - Authentication Cancelled</title><style>body{font-family:system-ui,sans-serif;text-align:center;padding:50px;background:#1e1e1e;color:#fff;}h1{color:#ff6b6b;}</style></head><body><h1>Authentication Cancelled</h1><p>%s</p><p>You can close this window and return to AI Gauge.</p></body></html>`, desc)))
		ls.sendResult("", fmt.Errorf("oauth provider error: %s", desc))
		return
	}

	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>AI Gauge - Authentication Error</title><style>body{font-family:system-ui,sans-serif;text-align:center;padding:50px;background:#1e1e1e;color:#fff;}h1{color:#ff6b6b;}</style></head><body><h1>Authentication Error</h1><p>No authorization code received.</p></body></html>`))
		ls.sendResult("", fmt.Errorf("no authorization code received"))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>AI Gauge - Connected</title><style>body{font-family:system-ui,sans-serif;text-align:center;padding:50px;background:#18181b;color:#f4f4f5;}h1{color:#4ade80;}p{color:#a1a1aa;}</style></head><body><h1>Connected!</h1><p>Authentication was successful. You can close this tab and return to AI Gauge.</p><script>setTimeout(() => window.close(), 2500);</script></body></html>`))
	ls.sendResult(code, nil)
}

func (ls *LoopbackServer) sendResult(code string, err error) {
	ls.once.Do(func() {
		ls.resultCh <- callbackResult{code: code, err: err}
	})
}

// WaitForCode waits for the OAuth authorization code or returns an error on context cancellation.
func (ls *LoopbackServer) WaitForCode(ctx context.Context) (string, error) {
	select {
	case res := <-ls.resultCh:
		return res.code, res.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Close shuts down the loopback server and frees the port.
func (ls *LoopbackServer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return ls.server.Shutdown(ctx)
}
