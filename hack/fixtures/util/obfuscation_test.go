package util

import "testing"

func TestObfuscate(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"changeme", "cegikmoq"},
		{"P@ssw0rd", "P@suw0ya"},
		{"Secret123!Hello456", "Segikm135!Uoqsu792"},
		{"sk-proj-AbCdEf123456", "su-wyac-AeCgEEXAMPLE"},
		{"6813ce57-b92d-46f8-c1eb-d357f9ce2b4d", "6802ce47-a91d-35f8-b0ea-c246fEXAMPLE"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := Obfuscate(tc.input)
			if got != tc.want {
				t.Errorf("Obfuscate(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
