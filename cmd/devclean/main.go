package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/sebavidal10/devclean/internal/disk"
	"github.com/sebavidal10/devclean/internal/plugins"
	"github.com/sebavidal10/devclean/internal/tui"
)

const banner = `
  ██████╗ ███████╗██╗   ██╗ ██████╗██╗     ███████╗ █████╗ ███╗   ██╗
  ██╔══██╗██╔════╝██║   ██║██╔════╝██║     ██╔════╝██╔══██╗████╗  ██║
  ██║  ██║█████╗  ██║   ██║██║     ██║     █████╗  ███████║██╔██╗ ██║
  ██║  ██║██╔══╝  ╚██╗ ██╔╝██║     ██║     ██╔══╝  ██╔══██║██║╚██╗██║
  ██████╔╝███████╗ ╚████╔╝ ╚██████╗███████╗███████╗██║  ██║██║ ╚████║
  ╚═════╝ ╚══════╝  ╚═══╝   ╚═════╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝`

var version = "dev"

func main() {
	scanOnly := flag.Bool("scan", false, "Run scan and print results in non-interactive CLI mode")
	noTUI := flag.Bool("no-tui", false, "Disable interactive TUI and output plain text table")
	jsonOutput := flag.Bool("json", false, "Output the scan report as JSON")
	home, _ := os.UserHomeDir()
	workspace := flag.String("workspace", filepath.Join(home, "Workspace"), "Directory to scan for inactive node_modules")
	inactiveDays := flag.Int("inactive-days", 30, "Minimum inactivity age for node_modules")
	flag.Parse()
	if *inactiveDays < 1 {
		fmt.Fprintln(os.Stderr, "--inactive-days must be at least 1")
		os.Exit(2)
	}
	tui.Version = displayVersion()

	// Initialize plugins registry
	registry := plugins.NewRegistry()
	registry.Register(plugins.NewXcodePlugin())
	registry.Register(plugins.NewDockerPlugin())
	registry.Register(plugins.NewNodePluginWithConfig(*workspace, *inactiveDays))
	registry.Register(plugins.NewSystemPlugin())

	// Detect if running in an interactive terminal
	isTerm := isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())

	if *scanOnly || *noTUI || *jsonOutput || !isTerm {
		runCLI(registry, *jsonOutput)
		return
	}

	// Interactive TUI mode
	p := tea.NewProgram(
		tui.NewModel(registry),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		// Fallback to CLI if TTY fails to open
		fmt.Fprintf(os.Stderr, "Nota: No se pudo iniciar el modo TUI interactivo (%v). Cambiando a modo consola:\n\n", err)
		runCLI(registry, false)
	}
}

func displayVersion() string {
	if version == "dev" || strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

type scanOutput struct {
	Version   string                 `json:"version"`
	Disk      *disk.DiskStats        `json:"disk,omitempty"`
	Reports   []plugins.PluginReport `json:"reports"`
	Error     string                 `json:"error,omitempty"`
	ElapsedMS int64                  `json:"elapsed_ms"`
}

func runCLI(reg *plugins.Registry, jsonOutput bool) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorCyan)
	subStyle := lipgloss.NewStyle().Foreground(tui.ColorCyan).Bold(true)
	authorStyle := lipgloss.NewStyle().Foreground(tui.ColorGrey).Italic(true)
	safeStyle := lipgloss.NewStyle().Foreground(tui.ColorYellow).Bold(true)
	sizeStyle := lipgloss.NewStyle().Foreground(tui.ColorYellow).Bold(true)

	// 1. APFS Storage Stats
	stats, err := disk.GetDiskUsage("/")
	if !jsonOutput {
		fmt.Println()
		fmt.Println(headerStyle.Render(banner))
		fmt.Println(subStyle.Render(displayVersion()+" · Intelligent Cleaner for macOS Developers") + " " + authorStyle.Render("by @sebavidal10 (github.com/sponsors/sebavidal10)"))
		fmt.Println()
	}
	if err == nil && !jsonOutput {
		barWidth := 30
		usedSlots := int((stats.UsedPercentage / 100.0) * float64(barWidth))
		if usedSlots > barWidth {
			usedSlots = barWidth
		}
		freeSlots := barWidth - usedSlots
		if freeSlots < 0 {
			freeSlots = 0
		}
		bar := tui.BarFilledStyle.Render(strings.Repeat("█", usedSlots)) +
			tui.BarEmptyStyle.Render(strings.Repeat("░", freeSlots))

		diskCard := fmt.Sprintf(
			"Punto de montaje: %s   Almacenamiento APFS: %s Total\n"+
				"Espacio Usado:    %-10s [%s]  %.1f%%\n"+
				"Espacio Libre:    %-10s",
			stats.Path, stats.TotalString(), stats.UsedString(), bar, stats.UsedPercentage, stats.FreeString(),
		)
		fmt.Println(tui.BoxCard.Render(diskCard))
	}

	// 2. Registry Scan
	if !jsonOutput {
		fmt.Printf("\n⚡ Escaneando entornos activos en paralelo (Docker, Xcode, Node, System)...\n\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	start := time.Now()
	reports, err := reg.ScanAll(ctx)
	elapsed := time.Since(start)
	if jsonOutput {
		out := scanOutput{Version: displayVersion(), Disk: stats, Reports: reports, ElapsedMS: elapsed.Milliseconds()}
		if err != nil {
			out.Error = err.Error()
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(out); encodeErr != nil {
			fmt.Fprintf(os.Stderr, "Error al escribir JSON: %v\n", encodeErr)
		}
		return
	}

	if err != nil && len(reports) == 0 {
		fmt.Fprintf(os.Stderr, "Error durante el escaneo: %v\n", err)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Advertencia: el escaneo fue parcial: %v\n", err)
	}

	// 3. Print Tabular Results
	fmt.Printf("Escaneo completado en %.2fs. Oportunidades de limpieza detectadas:\n\n", elapsed.Seconds())
	fmt.Printf("%-32s %-45s %-10s %-12s\n",
		subStyle.Render("CATEGORÍA / ENTORNO"),
		subStyle.Render("ARTEFACTO / RUTA"),
		subStyle.Render("EDAD"),
		subStyle.Render("TAMAÑO"),
	)
	fmt.Println(strings.Repeat("─", 102))

	var totalRecoverable int64
	var totalItems int

	for _, rep := range reports {
		if len(rep.Items) == 0 {
			continue
		}

		fmt.Printf("\n%s  %s\n", headerStyle.Render(rep.Category), authorStyle.Render("• "+rep.Title))
		fmt.Printf("  %s %s\n", safeStyle.Render("✔ Seguro:"), authorStyle.Render(rep.SafetyNote))

		for _, item := range rep.Items {
			totalRecoverable += item.SizeBytes
			totalItems++

			pathDisplay := item.Description
			if len(pathDisplay) > 43 {
				pathDisplay = "..." + pathDisplay[len(pathDisplay)-40:]
			}

			ageDisplay := "-"
			if item.LastModDays > 0 {
				ageDisplay = fmt.Sprintf("%dd ago", item.LastModDays)
			}

			fmt.Printf("  ├─ %-27s %-45s %-10s %s\n",
				item.ID,
				pathDisplay,
				ageDisplay,
				sizeStyle.Render(disk.FormatBytes(uint64(item.SizeBytes))),
			)
		}
	}

	fmt.Println(strings.Repeat("─", 102))
	summaryCard := fmt.Sprintf(
		"Total Recuperable: %s a través de %d objetivos detectados\n"+
			"Garantía Zero-Footgun activa: .git, .env* y bases de datos están protegidos.",
		disk.FormatBytes(uint64(totalRecoverable)),
		totalItems,
	)
	fmt.Println(tui.FocusedCard.Render(summaryCard))
	fmt.Println()
}
