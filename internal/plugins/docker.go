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
	return "Docker: Unused Images & Build Cache"
}

func (d *DockerPlugin) SafetyNote() string {
	return "Zero-Footgun: Volúmenes persistentes y contenedores están 100% blindados (cero riesgo para bases de datos como PostgreSQL, MongoDB, MySQL). Solo se limpian imágenes huérfanas sin contenedores y caché de build descartable."
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

type dockerImageEntry struct {
	ID           string `json:"ID"`
	Repository   string `json:"Repository"`
	Tag          string `json:"Tag"`
	Size         string `json:"Size"`
	CreatedAt    string `json:"CreatedAt"`
	CreatedSince string `json:"CreatedSince"`
	Containers   string `json:"Containers"`
}

// getUsedDockerImages returns a set of image references and IDs currently used by ANY container (running or stopped).
func getUsedDockerImages(ctx context.Context) (map[string]bool, error) {
	used := make(map[string]bool)
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.Image}}")
	out, err := cmd.Output()
	if err != nil {
		return used, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		used[line] = true
		raw := strings.TrimPrefix(line, "sha256:")
		used[raw] = true
		if len(raw) >= 12 {
			used[raw[:12]] = true
		}
		if idx := strings.Index(line, ":"); idx != -1 {
			used[line[:idx]] = true
		}
	}
	return used, nil
}

// parseDockerCreatedAtDays calculates how many days have elapsed since the image was created.
func parseDockerCreatedAtDays(createdAt string) int {
	clean := strings.TrimSpace(createdAt)
	parts := strings.Split(clean, " ")
	if len(parts) >= 3 {
		if t, err := time.Parse("2006-01-02 15:04:05 -0700", parts[0]+" "+parts[1]+" "+parts[2]); err == nil {
			days := int(time.Since(t).Hours() / 24)
			if days >= 0 {
				return days
			}
		}
	}
	if len(parts) > 0 {
		if t, err := time.ParseInLocation("2006-01-02", parts[0], time.Local); err == nil {
			days := int(time.Since(t).Hours() / 24)
			if days >= 0 {
				return days
			}
		}
	}
	return 0
}

func (d *DockerPlugin) Scan(parent context.Context) (PluginReport, error) {
	report := PluginReport{
		PluginID:   d.ID(),
		Category:   d.Category(),
		Title:      d.Name(),
		SafetyNote: d.SafetyNote(),
		Items:      make([]ItemDetail, 0),
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	// 1. Scan Build Cache using docker system df --format "{{json .}}"
	dfCmd := exec.CommandContext(ctx, "docker", "system", "df", "--format", "{{json .}}")
	dfOutput, err := dfCmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(dfOutput)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var entry dockerDFEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}

			// Strictly forbid and ignore volumes and containers - only inspect Build Cache
			if entry.Type == "Build Cache" {
				reclaimBytes := parseReclaimableBytes(entry.Reclaimable)
				if reclaimBytes == 0 && entry.Active == "0" {
					reclaimBytes = parseReclaimableBytes(entry.Size)
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
	}

	// 2. Query containers to strictly identify all in-use images
	usedImages, err := getUsedDockerImages(ctx)
	if err != nil {
		return report, nil
	}

	// 3. Scan images and filter for those with ZERO container attachments
	imgCmd := exec.CommandContext(ctx, "docker", "images", "-a", "--format", "{{json .}}")
	imgOutput, err := imgCmd.Output()
	if err != nil {
		return report, nil
	}

	seenImageIDs := make(map[string]bool)
	imgScanner := bufio.NewScanner(strings.NewReader(string(imgOutput)))
	for imgScanner.Scan() {
		line := strings.TrimSpace(imgScanner.Text())
		if line == "" {
			continue
		}

		var img dockerImageEntry
		if err := json.Unmarshal([]byte(line), &img); err != nil {
			continue
		}

		rawID := strings.TrimPrefix(img.ID, "sha256:")
		shortID := rawID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		if shortID == "" || seenImageIDs[shortID] {
			continue
		}

		// Double validation for image usage:
		// A. Check Containers count reported by docker
		if img.Containers != "" && img.Containers != "0" && img.Containers != "N/A" {
			continue
		}

		// B. Check against active & stopped containers from docker ps -a
		refName := img.Repository
		if img.Tag != "" && img.Tag != "<none>" {
			refName = img.Repository + ":" + img.Tag
		}

		if usedImages[rawID] || usedImages[shortID] || (refName != "" && usedImages[refName]) || (img.Repository != "" && usedImages[img.Repository]) {
			continue
		}

		seenImageIDs[shortID] = true
		sizeBytes := ParseHumanBytes(img.Size)
		days := parseDockerCreatedAtDays(img.CreatedAt)

		var desc string
		var path string
		if img.Repository == "<none>" || img.Repository == "" {
			desc = fmt.Sprintf("<none>:<none> [%s] (capa huérfana)", shortID)
			path = fmt.Sprintf("docker://image/%s", shortID)
		} else {
			desc = fmt.Sprintf("%s:%s (sin contenedor)", img.Repository, img.Tag)
			path = fmt.Sprintf("docker://image/%s:%s", img.Repository, img.Tag)
		}

		report.Items = append(report.Items, ItemDetail{
			ID:          "docker-img:" + shortID,
			Path:        path,
			Description: desc,
			SizeBytes:   sizeBytes,
			LastModDays: days,
		})
		report.TotalBytes += sizeBytes
	}

	return report, nil
}

// Clean executes safe prune operations strictly limited to unlinked images and builder cache.
// Explicitly NEVER touches volumes or containers.
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

	// 1. Prune Builder Cache if targeted
	if hasItem("docker-builder-cache") {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		cmd := exec.CommandContext(ctx, "docker", "builder", "prune", "-f")
		out, err := cmd.Output()
		cancel()
		if err != nil {
			return totalFreed, fmt.Errorf("failed to prune docker builder cache: %w", err)
		}
		totalFreed += parsePruneSpaceFreed(string(out))
	}

	// 2. If cleanAll: prune all unused images atomically with docker image prune -a -f
	// Zero-Footgun Guarantee: docker image prune only affects images unlinked from containers.
	// Persistent volumes and containers are NEVER touched!
	if cleanAll {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		cmd := exec.CommandContext(ctx, "docker", "image", "prune", "-a", "-f")
		out, err := cmd.Output()
		cancel()
		if err != nil {
			return totalFreed, fmt.Errorf("failed to prune unused docker images: %w", err)
		}
		totalFreed += parsePruneSpaceFreed(string(out))
		return totalFreed, nil
	}

	// 3. If specific items were selected:
	for _, id := range itemIDs {
		if strings.HasPrefix(id, "docker-img:") {
			imgID := strings.TrimPrefix(id, "docker-img:")

			// Inspect image size beforehand for telemetry
			var imgSize int64
			inspectCtx, inspectCancel := context.WithTimeout(context.Background(), 5*time.Second)
			inspectCmd := exec.CommandContext(inspectCtx, "docker", "inspect", imgID, "--format", "{{.Size}}")
			if sizeOut, err := inspectCmd.Output(); err == nil {
				imgSize, _ = strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64)
			}
			inspectCancel()

			// Safety: NO --force flag. Docker daemon will reject if any container is using it.
			rmCtx, rmCancel := context.WithTimeout(context.Background(), 20*time.Second)
			rmCmd := exec.CommandContext(rmCtx, "docker", "image", "rm", imgID)
			out, err := rmCmd.Output()
			rmCancel()
			if err != nil {
				return totalFreed, fmt.Errorf("failed to remove unlinked docker image %s: %w", imgID, err)
			}

			freed := parsePruneSpaceFreed(string(out))
			if freed > 0 {
				totalFreed += freed
			} else if imgSize > 0 {
				totalFreed += imgSize
			}
		} else if id == "docker-images-dangling" {
			// Backward compatibility with dangling image ID
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			cmd := exec.CommandContext(ctx, "docker", "image", "prune", "-f")
			out, err := cmd.Output()
			cancel()
			if err != nil {
				return totalFreed, fmt.Errorf("failed to prune dangling docker images: %w", err)
			}
			totalFreed += parsePruneSpaceFreed(string(out))
		}
	}

	return totalFreed, nil
}

// parseReclaimableBytes extracts byte value from strings like "18.3GB (62%)", "778.2kB", or "20.13GB".
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
