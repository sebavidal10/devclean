package plugins

import "testing"

func TestParseHumanBytes(t *testing.T) {
	tests := map[string]int64{
		"10B":    10,
		"1.5kB":  1500,
		"1KiB":   1024,
		"2MiB":   2 * 1024 * 1024,
		"1.2GB":  1_200_000_000,
		"3.4TiB": 3_738_339_534_438,
		"1..2GB": 0,
		"12XB":   0,
	}
	for input, expected := range tests {
		if got := ParseHumanBytes(input); got != expected {
			t.Errorf("ParseHumanBytes(%q) = %d; want %d", input, got, expected)
		}
	}
}

func TestParseDockerOutput(t *testing.T) {
	if got := parseReclaimableBytes("18.3GB (62%)"); got != 18_300_000_000 {
		t.Fatalf("unexpected reclaimable bytes: %d", got)
	}
	if got := parsePruneSpaceFreed("Total reclaimed space: 1.234GB\n"); got != 1_234_000_000 {
		t.Fatalf("unexpected reclaimed bytes: %d", got)
	}
}
