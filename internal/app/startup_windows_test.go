//go:build windows

package app

import "testing"

func TestStartupRuntimeInitializationResults(t *testing.T) {
	for _, tc := range []struct {
		hr               uint32
		acquired, failed bool
	}{
		{0, true, false}, {1, true, false}, {0x80010106, false, false}, {0x80004005, false, true},
	} {
		acquired, err := startupRuntimeResult(tc.hr)
		if acquired != tc.acquired || (err != nil) != tc.failed {
			t.Fatalf("HRESULT %x: acquired=%v error=%v", tc.hr, acquired, err)
		}
	}
}

func TestStartupEnableRejectsDisabledResults(t *testing.T) {
	for _, state := range []int32{0, 1, 2, 3, 4, 99} {
		err := checkStartupEnabled(state)
		enabled := state == 2 || state == 4
		if (err == nil) != enabled {
			t.Fatalf("state %d: error=%v", state, err)
		}
	}
}
