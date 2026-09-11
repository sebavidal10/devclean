package disk

import (
	"testing"
)

func TestGetDiskUsage(t *testing.T) {
	stats, err := GetDiskUsage("/")
	if err != nil {
		t.Fatalf("GetDiskUsage(/) returned error: %v", err)
	}

	if stats.TotalBytes == 0 {
		t.Errorf("expected TotalBytes > 0, got 0")
	}
	if stats.FreeBytes == 0 {
		t.Errorf("expected FreeBytes > 0, got 0")
	}
	if stats.TotalBytes < stats.FreeBytes {
		t.Errorf("TotalBytes (%d) should be >= FreeBytes (%d)", stats.TotalBytes, stats.FreeBytes)
	}
	if stats.UsedPercentage < 0 || stats.UsedPercentage > 100 {
		t.Errorf("UsedPercentage should be between 0 and 100, got %f", stats.UsedPercentage)
	}

	t.Logf("Disk Root Usage: %s Total, %s Used (%.1f%%), %s Free",
		stats.TotalString(), stats.UsedString(), stats.UsedPercentage, stats.FreeString())
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KiB"},
		{1024 * 1024, "1.00 MiB"},
		{1024 * 1024 * 1024, "1.00 GiB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s; want %s", tt.bytes, result, tt.expected)
		}
	}
}
