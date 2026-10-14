package tomlutil

import "testing"

func TestKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"bare", "node", "node"},
		{"bare with dash and underscore", "some-key_2", "some-key_2"},
		{"empty", "", `""`},
		{"scoped", "npm:playwright", `"npm:playwright"`},
		{"dotted", "a.b", `"a.b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Key(tt.key); got != tt.want {
				t.Errorf("Key() = %v, want %v", got, tt.want)
			}
		})
	}
}
