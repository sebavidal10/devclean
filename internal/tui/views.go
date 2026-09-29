package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sebavidal10/devclean/internal/disk"
)

const banner = `
  ██████╗ ███████╗██╗   ██╗ ██████╗██╗     ███████╗ █████╗ ███╗   ██╗
  ██╔══██╗██╔════╝██║   ██║██╔════╝██║     ██╔════╝██╔══██╗████╗  ██║
  ██║  ██║█████╗  ██║   ██║██║     ██║     █████╗  ███████║██╔██╗ ██║
  ██║  ██║██╔══╝  ╚██╗ ██╔╝██║     ██║     ██╔══╝  ██╔══██║██║╚██╗██║
  ██████╔╝███████╗ ╚████╔╝ ╚██████╗███████╗███████╗██║  ██║██║ ╚████║
  ╚═════╝ ╚══════╝  ╚═══╝   ╚═════╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝`

// cardInnerWidth calculates responsive inner width for cards based on terminal window dimensions.
func (m Model) cardInnerWidth() int {
	w := m.width - 6
	if w < 74 {
		return 74
	}
	if w > 160 {
		return 160
	}
	return w
}

func (m Model) renderHeader() string {
	var b strings.Builder

	// Banner
	b.WriteString(BannerStyle.Render(banner))
	b.WriteString("\n")

	// Subtitle & Sponsor Credit
	b.WriteString(SubtitleStyle.Render(Version + " · Intelligent Cleaner for macOS Developers"))
	b.WriteString("  ")
	b.WriteString(AuthorSponsorStyle.Render("by @sebavidal10 (github.com/sponsors/sebavidal10)"))
	b.WriteString("\n\n")

	// Initial APFS Disk Bar
	if m.initialDisk != nil {
		b.WriteString(m.renderDiskBar(m.initialDisk, "Estado del disco actual"))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderDiskBar(d *disk.DiskStats, label string) string {
	innerW := m.cardInnerWidth()
	barWidth := innerW - 54
	if barWidth < 20 {
		barWidth = 20
	}
	if barWidth > 50 {
		barWidth = 50
	}

	usedSlots := int((d.UsedPercentage / 100.0) * float64(barWidth))
	if usedSlots > barWidth {
		usedSlots = barWidth
	}
	freeSlots := barWidth - usedSlots
	if freeSlots < 0 {
		freeSlots = 0
	}

	bar := BarFilledStyle.Render(strings.Repeat("█", usedSlots)) +
		BarEmptyStyle.Render(strings.Repeat("░", freeSlots))

	cardContent := fmt.Sprintf(
		"%s: %s [%s]  %.1f%% usado  (%s libres de %s)",
		CategoryTitleFocused.Render(label),
		d.UsedString(),
		bar,
		d.UsedPercentage,
		d.FreeString(),
		d.TotalString(),
	)

	return BoxCard.Copy().Width(innerW).Render(cardContent)
}

func (m Model) viewScanning() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	innerW := m.cardInnerWidth()

	spinBox := fmt.Sprintf(
		"%s Escaneando entornos activos en paralelo (Docker, Xcode, Node, System)...\n%s",
		m.spinner.View(),
		SafetyText.Render("Analizando DerivedData, simuladores, Docker (imágenes huérfanas y build cache), npm y logs sin alterar datos persistentes."),
	)
	b.WriteString(FocusedCard.Copy().Width(innerW).Render(spinBox))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewSelection() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	innerW := m.cardInnerWidth()

	if len(m.categories) == 0 {
		emptyBox := BoxCard.Copy().Width(innerW).Render("No se detectaron artefactos de compilación ni cachés descartables.\n¡Tu Mac está reluciente!")
		b.WriteString(emptyBox)
		b.WriteString("\n\n" + m.renderFooter("q: Salir"))
		return b.String()
	}

	var totalDetected int64
	var totalSelected int64

	for i, cat := range m.categories {
		isCursor := i == m.cursor
		totalDetected += cat.Report.TotalBytes
		totalSelected += cat.SelectedBytes()

		cursorMarker := "  "
		if isCursor {
			cursorMarker = CursorStyle.Render("❯ ")
		}

		checkMarker := CheckboxUnchecked.Render("[ ]")
		if cat.Selected {
			checkMarker = CheckboxChecked.Render("[x]")
		}

		titleStyle := CategoryTitle
		if isCursor {
			titleStyle = CategoryTitleFocused
		}

		catTitle := cat.Report.Title
		sizeStr := disk.FormatBytes(uint64(cat.Report.TotalBytes))
		selectedCount := fmt.Sprintf("(%d/%d seleccionados)", cat.SelectedCount(), len(cat.Report.Items))

		// Dynamic spacing: calculate space between category title and right metadata (size + count)
		rightPartLen := len(sizeStr) + 2 + len(selectedCount)
		availTitleW := innerW - 6 - rightPartLen - 2
		if availTitleW < 24 {
			availTitleW = 24
		}

		titlePad := ""
		if len(catTitle) < availTitleW {
			titlePad = strings.Repeat(" ", availTitleW-len(catTitle))
		} else if len(catTitle) > availTitleW {
			catTitle = catTitle[:availTitleW-3] + "..."
		}

		headerLine := fmt.Sprintf("%s%s %s%s  %s  %s",
			cursorMarker,
			checkMarker,
			titleStyle.Render(catTitle),
			titlePad,
			SizeTagStyle.Render(sizeStr),
			AuthorSponsorStyle.Render(selectedCount),
		)

		safetyLine := fmt.Sprintf("     %s %s",
			SafetyBadge.Render("✔ SEGURO:"),
			SafetyText.Render(cat.Report.SafetyNote),
		)

		itemBlock := headerLine + "\n" + safetyLine

		cardStyle := BoxCard.Copy().Width(innerW)
		if isCursor {
			cardStyle = FocusedCard.Copy().Width(innerW)
		}
		b.WriteString(cardStyle.Render(itemBlock))
		b.WriteString("\n")
	}

	summaryLine := fmt.Sprintf(
		"Seleccionado: %s de %s detectados",
		RecoveredStat.Render(disk.FormatBytes(uint64(totalSelected))),
		disk.FormatBytes(uint64(totalDetected)),
	)
	b.WriteString(StatusBar.Render(summaryLine))
	b.WriteString("\n\n")

	if m.statusMessage != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorYellow).Render("⚠ "+m.statusMessage) + "\n\n")
	}

	keys := []string{
		KeyStyle.Render("↑/k ↓/j") + KeyDescStyle.Render(": Navegar"),
		KeyStyle.Render("space") + KeyDescStyle.Render(": Marcar"),
		KeyStyle.Render("a") + KeyDescStyle.Render(": Todo"),
		KeyStyle.Render("d/enter") + KeyDescStyle.Render(": Detalle"),
		KeyStyle.Render("c") + KeyDescStyle.Render(": Limpiar"),
		KeyStyle.Render("q") + KeyDescStyle.Render(": Salir"),
	}
	b.WriteString(strings.Join(keys, "  •  "))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewDrillDown() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())

	if len(m.categories) == 0 {
		return m.viewSelection()
	}

	cat := &m.categories[m.cursor]
	innerW := m.cardInnerWidth()

	title := fmt.Sprintf("Detalle de Categoría: %s (%d items - %s seleccionados)",
		cat.Report.Title,
		len(cat.Report.Items),
		disk.FormatBytes(uint64(cat.SelectedBytes())),
	)
	b.WriteString(CategoryTitleFocused.Render(title))
	b.WriteString("\n\n")

	// Calculate responsive column width for item description
	// Fixed columns: cursor (2) + check (4) + spacing (2) + age (10) + spacing (2) + size (12) = 32
	descWidth := innerW - 32
	if descWidth < 30 {
		descWidth = 30
	}

	var listBuilder strings.Builder
	for i, item := range cat.Report.Items {
		isCursor := i == m.itemCursor

		cursorMarker := "  "
		if isCursor {
			cursorMarker = CursorStyle.Render("❯ ")
		}

		checkMarker := CheckboxUnchecked.Render("[ ]")
		if cat.SelectedItems[item.ID] {
			checkMarker = CheckboxChecked.Render("[x]")
		}

		desc := item.Description
		if len(desc) > descWidth {
			if descWidth > 3 {
				desc = desc[:descWidth-3] + "..."
			}
		}
		descPad := ""
		if len(desc) < descWidth {
			descPad = strings.Repeat(" ", descWidth-len(desc))
		}

		age := "-"
		if item.LastModDays > 0 {
			age = fmt.Sprintf("%dd ago", item.LastModDays)
		}
		agePad := ""
		if len(age) < 10 {
			agePad = strings.Repeat(" ", 10-len(age))
		}

		sizeStr := disk.FormatBytes(uint64(item.SizeBytes))
		sizePad := ""
		if len(sizeStr) < 12 {
			sizePad = strings.Repeat(" ", 12-len(sizeStr))
		}

		line := fmt.Sprintf("%s%s %s%s  %s%s  %s%s",
			cursorMarker,
			checkMarker,
			ItemPathStyle.Render(desc),
			descPad,
			ItemAgeStyle.Render(age),
			agePad,
			sizePad,
			SizeTagStyle.Render(sizeStr),
		)
		listBuilder.WriteString(line + "\n")
	}

	cardContent := strings.TrimRight(listBuilder.String(), "\n")
	b.WriteString(BoxCard.Copy().Width(innerW).Render(cardContent))
	b.WriteString("\n\n")

	keys := []string{
		KeyStyle.Render("↑/k ↓/j") + KeyDescStyle.Render(": Navegar"),
		KeyStyle.Render("space") + KeyDescStyle.Render(": Marcar"),
		KeyStyle.Render("a") + KeyDescStyle.Render(": Todo"),
		KeyStyle.Render("esc/b") + KeyDescStyle.Render(": Volver"),
		KeyStyle.Render("q") + KeyDescStyle.Render(": Salir"),
	}
	b.WriteString(strings.Join(keys, "  •  "))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewCleaning() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	innerW := m.cardInnerWidth()

	spinBox := fmt.Sprintf(
		"%s %s\n\n%s",
		m.spinner.View(),
		CategoryTitleFocused.Render(m.cleanStatus),
		SafetyText.Render("Zero-Footgun: Ejecutando en espacio de usuario. Nunca se alteran repositorios .git ni variables de entorno."),
	)
	b.WriteString(FocusedCard.Copy().Width(innerW).Render(spinBox))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewSummary() string {
	var b strings.Builder
	innerW := m.cardInnerWidth()

	b.WriteString(BannerStyle.Render(banner))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render(Version + " · Intelligent Cleaner for macOS Developers\n\n"))

	headerText := "✨ ¡Limpieza completada con éxito!"
	if m.err != nil {
		headerText = "⚠ Limpieza completada parcialmente"
	}
	header := SuccessTitle.Render(headerText)
	freedMetric := RecoveredStat.Render(disk.FormatBytes(uint64(m.freedBytes))) + " recuperados"

	summaryText := fmt.Sprintf(
		"%s\n\nEspacio total liberado: %s\n",
		header,
		freedMetric,
	)

	if m.initialDisk != nil && m.finalDisk != nil {
		summaryText += "\n" + m.renderDotDiskComparison(m.initialDisk, m.finalDisk)
	}
	if m.err != nil {
		summaryText += "\n" + lipgloss.NewStyle().Foreground(ColorYellow).Render("Problemas encontrados: "+m.err.Error()) + "\n"
	}

	summaryText += "\n" + SafetyText.Render("Zero-Footgun Guarantee: Tus proyectos, configuraciones y datos persistentes están intactos.")
	summaryText += "\n\n" + SponsorCallout.Render("❤ ¿Te fue útil devclean? Considera apoyar el proyecto en github.com/sponsors/sebavidal10")

	b.WriteString(FocusedCard.Copy().Width(innerW).Render(summaryText))
	b.WriteString("\n\n")

	b.WriteString(KeyStyle.Render("Presiona [q] o [Enter] para salir."))
	b.WriteString("\n")

	return b.String()
}

func (m Model) renderDotDiskComparison(before, after *disk.DiskStats) string {
	innerW := m.cardInnerWidth()
	dotWidth := (innerW - 40) / 2
	if dotWidth < 15 {
		dotWidth = 15
	}
	if dotWidth > 35 {
		dotWidth = 35
	}

	makeDots := func(d *disk.DiskStats) string {
		usedDots := int((d.UsedPercentage / 100.0) * float64(dotWidth))
		if usedDots > dotWidth {
			usedDots = dotWidth
		}
		freeDots := dotWidth - usedDots
		if freeDots < 0 {
			freeDots = 0
		}
		return BarFilledStyle.Render(strings.Repeat("●", usedDots)) +
			BarEmptyStyle.Render(strings.Repeat("○", freeDots))
	}

	return fmt.Sprintf(
		"Antes: [%s] %.1f%% usado (%s libres)\n"+
			"Ahora: [%s] %.1f%% usado (%s libres)\n",
		makeDots(before),
		before.UsedPercentage,
		before.FreeString(),
		makeDots(after),
		after.UsedPercentage,
		after.FreeString(),
	)
}

func (m Model) renderFooter(keys ...string) string {
	return KeyDescStyle.Render(strings.Join(keys, "  •  "))
}
