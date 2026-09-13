package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DockerPlugin struct{}

func NewDockerPlugin() *DockerPlugin {
	return &DockerPlugin{}
}

func (d *DockerPlugin) ID() string {
	return "docker"
}

func (d *DockerPlugin) Category() string {
	return "Containers & Virtualization"
}

func (d *DockerPlugin) Name() string {
	return "Docker Dangling Layers & Build Cache"
}

func (d *DockerPlugin) SafetyNote() string {
	return "Zero-Footgun Guarantee: Only untagged/dangling images and builder cache are scanned. Named images, running containers, and volumes are NEVER touched."
}

func (d *DockerPlugin) Detect() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

type dockerDFEntry struct {
	Type        string `json:"Type"`
	TotalCount  string `json:"TotalCount"`
	Active      string `json:"Active"`
	Size        string `json:"Size"`
	Reclaimable string `json:"Reclaimable"`
}

func (d *DockerPlugin) Scan(parent context.Context) (PluginReport, error) {
	report := PluginReport{
		PluginID:   d.ID(),
		Category:   d.Category(),
		Title:      d.Name(),
		SafetyNote: d.SafetyNote(),
		Items:      make([]ItemDetail, 0),
	}

	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()

	// Run safe inspection command: docker system df --format "{{json .}}"
	cmd := exec.CommandContext(ctx, "docker", "system", "df", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return report, fmt.Errorf("docker system df failed: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry dockerDFEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Strictly forbid and ignore volumes and containers
		switch entry.Type {
		case "Images":
			reclaimBytes := parseReclaimableBytes(entry.Reclaimable)
			if reclaimBytes > 0 {
				report.Items = append(report.Items, ItemDetail{
					ID:          "docker-images-dangling",
					Path:        "docker://images/dangling",
					Description: fmt.Sprintf("Dangling/Untagged Images (%s reclaimable)", entry.Reclaimable),
					SizeBytes:   reclaimBytes,
					LastModDays: 0,
				})
				report.TotalBytes += reclaimBytes
			}
		case "Build Cache":
			reclaimBytes := parseReclaimableBytes(entry.Reclaimable)
			if reclaimBytes == 0 {
				// If reclaimable says 0B or unparsed, check total cache size if active is 0
				if entry.Active == "0" {
					reclaimBytes = parseReclaimableBytes(entry.Size)
				}
			}
			if reclaimBytes > 0 {
				report.Items = append(report.Items, ItemDetail{
					ID:          "docker-builder-cache",
					Path:        "docker://builder/cache",
					Description: fmt.Sprintf("Docker Builder Cache (%s)", entry.Size),
					SizeBytes:   reclaimBytes,
					LastModDays: 0,
				})
				report.TotalBytes += reclaimBytes
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return report, fmt.Errorf("read docker system df output: %w", err)
	}

	return report, nil
}

// Clean executes safe prune operations strictly limited to dangling images and builder cache.
// Explicitly prohibits passing -a and NEVER touches volumes.
func (d *DockerPlugin) Clean(itemIDs []string) (int64, error) {
	var totalFreed int64
	cleanAll := len(itemIDs) == 0

	hasItem := func(id string) bool {
		if cleanAll {
			return true
		}
		for _, item := range itemIDs {
			if item == id {
				return true
			}
		}
		return false
	}

	// 1. Prune dangling images ONLY (NO -a flag)
	if hasItem("docker-images-dangling") {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, "docker", "image", "prune", "-f")
		out, err := cmd.Output()
		cancel()
		if err != nil {
			return totalFreed, fmt.Errorf("failed to prune dangling docker images: %w", err)
		}
		totalFreed += parsePruneSpaceFreed(string(out))
	}

	// 2. Prune builder cache ONLY
	if hasItem("docker-builder-cache") {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, "docker", "builder", "prune", "-f")
		out, err := cmd.Output()
		cancel()
		if err != nil {
			return totalFreed, fmt.Errorf("failed to prune docker builder cache: %w", err)
		}
		totalFreed += parsePruneSpaceFreed(string(out))
	}

	return totalFreed, nil
}

// parseReclaimableBytes extracts byte value from string like "18.3GB (62%)", "778.2kB", or "20.13GB".
func parseReclaimableBytes(raw string) int64 {
	clean := strings.TrimSpace(raw)
	if idx := strings.Index(clean, "("); idx != -1 {
		clean = strings.TrimSpace(clean[:idx])
	}
	return ParseHumanBytes(clean)
}

// parsePruneSpaceFreed parses "Total reclaimed space: 1.234GB" from docker prune stdout.
func parsePruneSpaceFreed(output string) int64 {
	re := regexp.MustCompile(`Total reclaimed space:\s*([0-9\.]+\s*[a-zA-Z]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return ParseHumanBytes(matches[1])
	}
	return 0
}

// ParseHumanBytes parses strings like "10B", "15KB", "20.5MB", "1.2GB", "3.4TiB" into bytes.
func ParseHumanBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0B" || s == "0" {
		return 0
	}

	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([a-zA-Z]+)$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) < 3 {
		return 0
	}

	val, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := strings.ToUpper(matches[2])
	var multiplier float64

	switch unit {
	case "B":
		multiplier = 1
	case "KB", "K":
		multiplier = 1000
	case "KIB":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1000 * 1000
	case "MIB":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1000 * 1000 * 1000
	case "GIB":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "TIB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0
	}

	return int64(val * multiplier)
}
