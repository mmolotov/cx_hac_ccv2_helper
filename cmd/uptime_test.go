package cmd

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{1*time.Hour + 5*time.Minute + 3*time.Second, "1h 5m 3s"},
		{25*time.Hour + 30*time.Minute, "1d 1h 30m 0s"},
		{48 * time.Hour, "2d 0s"},
	}
	for _, tc := range cases {
		got := formatDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
