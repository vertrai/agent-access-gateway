package telegram

import (
	"errors"
	"testing"
	"time"
)

func TestMaskToken(t *testing.T) {
	got := MaskToken("123456:abcdefghijklmnopqrstuvwxyz")
	if got != "123456:abc***xyz" {
		t.Fatalf("MaskToken() = %q", got)
	}
}

func TestParseFloodWaitSeconds(t *testing.T) {
	for _, test := range []struct {
		err  error
		want int
		ok   bool
	}{{errors.New("FLOOD_WAIT (192)"), 192, true}, {errors.New("FLOOD_WAIT_70"), 70, true}, {errors.New("USERNAME_INVALID"), 0, false}} {
		got, ok := parseFloodWaitSeconds(test.err)
		if got != test.want || ok != test.ok {
			t.Fatalf("parseFloodWaitSeconds(%v) = (%d, %v)", test.err, got, ok)
		}
	}
}

func TestPositiveRemaining(t *testing.T) {
	now := time.Now()
	if positiveRemaining(now.Add(-time.Second), now) != 0 || positiveRemaining(time.Time{}, now) != 0 {
		t.Fatal("past and zero cooldowns must be inactive")
	}
	if positiveRemaining(now.Add(time.Second), now) <= 0 {
		t.Fatal("future cooldown must be active")
	}
}

func TestIsTelegram2FARequired(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{err: errors.New("callback: 2FA required"), want: true},
		{err: errors.New("rpc error: SESSION_PASSWORD_NEEDED"), want: true},
		{err: errors.New("PHONE_CODE_INVALID"), want: false},
		{err: nil, want: false},
	}
	for _, test := range tests {
		if got := isTelegram2FARequired(test.err); got != test.want {
			t.Fatalf("isTelegram2FARequired(%v) = %v, want %v", test.err, got, test.want)
		}
	}
}

func TestTokenPattern(t *testing.T) {
	if !tokenPattern.MatchString("123456:abcdefghijklmnopqrstuvwxyz") || tokenPattern.MatchString("bad") {
		t.Fatal("unexpected token validation")
	}
}
