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
	barWidth := 28
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

	return BoxCard.Render(cardContent)
}

func (m Model) viewScanning() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())

	spinBox := fmt.Sprintf(
		"%s Escaneando entornos activos en paralelo (Docker, Xcode, Node, System)...\n%s",
		m.spinner.View(),
		SafetyText.Render("Analizando DerivedData, simuladores, Docker dangling, npm y logs sin alterar datos persistentes."),
	)
	b.WriteString(FocusedCard.Render(spinBox))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewSelection() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())

	if len(m.categories) == 0 {
		emptyBox := BoxCard.Render("No se detectaron artefactos de compilación ni cachés descartables.\n¡Tu Mac está reluciente!")
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

		catTitle := titleStyle.Render(cat.Report.Title)
		sizeStr := SizeTagStyle.Render(disk.FormatBytes(uint64(cat.Report.TotalBytes)))
		selectedCount := fmt.Sprintf("(%d/%d seleccionados)", cat.SelectedCount(), len(cat.Report.Items))

		headerLine := fmt.Sprintf("%s%s %-36s %12s  %s",
			cursorMarker,
			checkMarker,
			catTitle,
			sizeStr,
			AuthorSponsorStyle.Render(selectedCount),
		)

		safetyLine := fmt.Sprintf("     %s %s",
			SafetyBadge.Render("✔ SEGURO:"),
			SafetyText.Render(cat.Report.SafetyNote),
		)

		itemBlock := headerLine + "\n" + safetyLine

		if isCursor {
			b.WriteString(FocusedCard.Render(itemBlock))
		} else {
			b.WriteString(BoxCard.Render(itemBlock))
		}
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

	title := fmt.Sprintf("Detalle de Categoría: %s (%d items - %s seleccionados)",
		cat.Report.Title,
		len(cat.Report.Items),
		disk.FormatBytes(uint64(cat.SelectedBytes())),
	)
	b.WriteString(CategoryTitleFocused.Render(title))
	b.WriteString("\n\n")

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
		if len(desc) > 42 {
			desc = "..." + desc[len(desc)-39:]
		}

		age := "-"
		if item.LastModDays > 0 {
			age = fmt.Sprintf("%dd ago", item.LastModDays)
		}

		line := fmt.Sprintf("%s%s %-44s %-10s %s",
			cursorMarker,
			checkMarker,
			ItemPathStyle.Render(desc),
			ItemAgeStyle.Render(age),
			SizeTagStyle.Render(disk.FormatBytes(uint64(item.SizeBytes))),
		)
		listBuilder.WriteString(line + "\n")
	}

	b.WriteString(BoxCard.Render(listBuilder.String()))
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

	spinBox := fmt.Sprintf(
		"%s %s\n\n%s",
		m.spinner.View(),
		CategoryTitleFocused.Render(m.cleanStatus),
		SafetyText.Render("Zero-Footgun: Ejecutando en espacio de usuario. Nunca se alteran repositorios .git ni variables de entorno."),
	)
	b.WriteString(FocusedCard.Render(spinBox))
	b.WriteString("\n")

	return b.String()
}

func (m Model) viewSummary() string {
	var b strings.Builder

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

	b.WriteString(FocusedCard.Render(summaryText))
	b.WriteString("\n\n")

	b.WriteString(KeyStyle.Render("Presiona [q] o [Enter] para salir."))
	b.WriteString("\n")

	return b.String()
}

func (m Model) renderDotDiskComparison(before, after *disk.DiskStats) string {
	dotWidth := 20

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
