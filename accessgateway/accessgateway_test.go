package accessgateway

import "testing"

func TestNew(t *testing.T) {
	g := New("test", Config{}, nil)
	if g.env != "test" || g.config.BrowserTimeoutMinutes != 240 {
		t.Fatalf("unexpected defaults: %+v", g.config)
	}
}
