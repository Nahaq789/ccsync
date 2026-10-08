package syncer

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		name   string
		local  string
		stored string
		want   State
	}{
		{"同じ内容", "A\nB\n", "A\nB\n", Synced},
		{"どちらも空", "", "", Synced},
		{"この PC の方が、続きがある", "A\nB\nC\n", "A\nB\n", LocalAhead},
		{"ストアの方が、続きがある", "A\nB\n", "A\nB\nC\n", StoreAhead},
		{"途中から内容が分かれている", "A\nB\nX\n", "A\nB\nY\n", Conflict},
		{"最初から内容が違う", "X\n", "Y\n", Conflict},
		{"長さは同じで、内容が違う", "A\nB\n", "A\nC\n", Conflict},
		{"ストアが空で、この PC にはある", "A\n", "", LocalAhead},
		{"この PC が空で、ストアにはある", "", "A\n", StoreAhead},
		{"先頭は同じだが、途中の行が違う", "A\nB\nC\n", "A\nX\n", Conflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Compare([]byte(tt.local), []byte(tt.stored)); got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.local, tt.stored, got, tt.want)
			}
		})
	}
}
