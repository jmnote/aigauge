package providers

import "testing"

func TestLimitedBufferKeepsThePrefixAndReportsFullWrite(t *testing.T) {
	var got limitedBuffer
	got.limit = 4

	n, err := got.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 6 {
		t.Fatalf("Write() count = %d, want 6", n)
	}
	if got.String() != "abcd" {
		t.Fatalf("String() = %q, want %q", got.String(), "abcd")
	}

	n, err = got.Write([]byte("!"))
	if err != nil || n != 1 {
		t.Fatalf("second Write() = (%d, %v), want (1, nil)", n, err)
	}
	if got.String() != "abcd" {
		t.Fatalf("String() after full buffer = %q, want %q", got.String(), "abcd")
	}
}
