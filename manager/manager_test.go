package manager

import "testing"

func TestNew(t *testing.T) {
	if New("test", Config{}) == nil {
		t.Fatal("New returned nil")
	}
}
