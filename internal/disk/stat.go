package disk

import (
	"fmt"
	"math"

	"golang.org/x/sys/unix"
)

// DiskStats represents file system usage metrics for a given mount path.
type DiskStats struct {
	Path           string  `json:"path"`
	TotalBytes     uint64  `json:"total_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPercentage float64 `json:"used_percentage"`
}

// GetDiskUsage retrieves native APFS/HFS+ file system stats on macOS using unix.Statfs.
func GetDiskUsage(path string) (*DiskStats, error) {
	if path == "" {
		path = "/"
	}

	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("statfs failed for path %q: %w", path, err)
	}

	blockSize := uint64(stat.Bsize)
	totalBytes := stat.Blocks * blockSize
	freeBytes := stat.Bavail * blockSize

	var usedBytes uint64
	if totalBytes >= freeBytes {
		usedBytes = totalBytes - freeBytes
	}

	var usedPct float64
	if totalBytes > 0 {
		usedPct = (float64(usedBytes) / float64(totalBytes)) * 100.0
	}

	return &DiskStats{
		Path:           path,
		TotalBytes:     totalBytes,
		FreeBytes:      freeBytes,
		UsedBytes:      usedBytes,
		UsedPercentage: math.Round(usedPct*100) / 100,
	}, nil
}

// FormatBytes converts a byte count into a human-readable string with binary units (GiB, MiB, etc.).
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	if exp >= len(suffixes) {
		exp = len(suffixes) - 1
	}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), suffixes[exp])
}

// TotalString returns human-readable total disk size.
func (d *DiskStats) TotalString() string {
	return FormatBytes(d.TotalBytes)
}

// FreeString returns human-readable free disk size.
func (d *DiskStats) FreeString() string {
	return FormatBytes(d.FreeBytes)
}

// UsedString returns human-readable used disk size.
func (d *DiskStats) UsedString() string {
	return FormatBytes(d.UsedBytes)
}

// TotalGiB returns the total storage in gibibytes.
func (d *DiskStats) TotalGiB() float64 {
	return float64(d.TotalBytes) / (1024 * 1024 * 1024)
}

// FreeGiB returns available storage in gibibytes.
func (d *DiskStats) FreeGiB() float64 {
	return float64(d.FreeBytes) / (1024 * 1024 * 1024)
}

// UsedGiB returns used storage in gibibytes.
func (d *DiskStats) UsedGiB() float64 {
	return float64(d.UsedBytes) / (1024 * 1024 * 1024)
}
