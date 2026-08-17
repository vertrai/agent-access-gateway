package manager

import "testing"

func TestNew(t *testing.T) {
	if service, err := New("test", Config{}, nil); err != nil || service == nil {
		t.Fatalf("New returned service=%v err=%v", service, err)
	}
}
