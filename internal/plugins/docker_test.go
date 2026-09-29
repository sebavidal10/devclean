package plugins

import (
	"strings"
	"testing"
	"time"
)

func TestParseHumanBytes(t *testing.T) {
	tests := map[string]int64{
		"10B":     10,
		"1.5kB":   1500,
		"1KiB":    1024,
		"2MiB":    2 * 1024 * 1024,
		"1.2GB":   1_200_000_000,
		"3.4TiB":  3_738_339_534_438,
		"1..2GB":  0,
		"12XB":    0,
		"33.64kB": 33_640,
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
	if got := parsePruneSpaceFreed("Total reclaimed space: 33.64kB\n"); got != 33_640 {
		t.Fatalf("unexpected reclaimed bytes: %d", got)
	}
	if got := parsePruneSpaceFreed("Total reclaimed space: 0B\n"); got != 0 {
		t.Fatalf("unexpected reclaimed bytes: %d", got)
	}
}

func TestParseDockerCreatedAtDays(t *testing.T) {
	// 5 days ago
	fiveDaysAgo := time.Now().AddDate(0, 0, -5).Format("2006-01-02 15:04:05 -0700 MST")
	if days := parseDockerCreatedAtDays(fiveDaysAgo); days != 5 {
		t.Errorf("expected 5 days, got %d", days)
	}

	// Invalid date
	if days := parseDockerCreatedAtDays("not-a-date"); days != 0 {
		t.Errorf("expected 0 days for invalid date, got %d", days)
	}
}

func TestDockerPluginMetadataAndSafety(t *testing.T) {
	p := NewDockerPlugin()
	if p.ID() != "docker" {
		t.Errorf("expected ID 'docker', got %s", p.ID())
	}
	if !strings.Contains(p.SafetyNote(), "blindados") || !strings.Contains(p.SafetyNote(), "Volúmenes") {
		t.Errorf("SafetyNote must explicitly mention protected volumes: %s", p.SafetyNote())
	}
	if !strings.Contains(p.Name(), "Unused Images") && !strings.Contains(p.Name(), "Imágenes") {
		t.Errorf("Name should clarify unused images: %s", p.Name())
	}
}
