package session

import "testing"

func TestEncodePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/home/naha/dev/pob", "-home-naha-dev-pob"},
		{"/home/naha/my.app", "-home-naha-my-app"},
		{"/home/naha/a_b", "-home-naha-a-b"},
		{"/home/naha/my app", "-home-naha-my-app"},
		{"/home/naha/a-b", "-home-naha-a-b"},
		{"/home/naha/.config/x", "-home-naha--config-x"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := EncodePath(tt.in); got != tt.want {
			t.Errorf("EncodePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
