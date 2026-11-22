package youtube

import "testing"

func TestParseDuration(t *testing.T) {
	cases := map[string]int{
		"PT4M13S":   253,
		"PT59S":     59,
		"PT1H":      3600,
		"PT1H2M3S":  3723,
		"PT10M":     600,
		"P1DT2H3S":  93603,
		"P1D":       86400,
		"P1W":       604800,
		"PT0S":      0, // live streams
		"P0D":       0,
		"":          0,
		"4:13":      0,
		"PT1.5S":    0,
		"PT12H0M5S": 43205,
	}
	for in, want := range cases {
		if got := parseDuration(in); got != want {
			t.Errorf("parseDuration(%q) = %d, want %d", in, got, want)
		}
	}
}
