package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sebavidal10/devclean/internal/disk"
	"github.com/sebavidal10/devclean/internal/plugins"
)

type AppState int

const (
	StateScanning AppState = iota
	StateSelection
	StateDrillDown
	StateCleaning
	StateSummary
	StateError
)

type CategoryItem struct {
	Report        plugins.PluginReport
	Selected      bool
	SelectedItems map[string]bool
}

func (c *CategoryItem) SelectedBytes() int64 {
	var total int64
	for _, item := range c.Report.Items {
		if c.SelectedItems[item.ID] {
			total += item.SizeBytes
		}
	}
	return total
}

func (c *CategoryItem) SelectedCount() int {
	var count int
	for _, item := range c.Report.Items {
		if c.SelectedItems[item.ID] {
			count++
		}
	}
	return count
}

type Model struct {
	state          AppState
	registry       *plugins.Registry
	initialDisk    *disk.DiskStats
	finalDisk      *disk.DiskStats
	spinner        spinner.Model
	categories     []CategoryItem
	cursor         int
	itemCursor     int
	cleanStatus    string
	freedBytes     int64
	err            error
	width          int
	height         int
	statusMessage  string
	confirmPending bool
}

func NewModel(reg *plugins.Registry) Model {
	if reg == nil {
		reg = plugins.NewRegistry()
		reg.Register(plugins.NewXcodePlugin())
		reg.Register(plugins.NewDockerPlugin())
		reg.Register(plugins.NewNodePlugin())
		reg.Register(plugins.NewSystemPlugin())
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorCyan)

	dStats, _ := disk.GetDiskUsage("/")

	return Model{
		state:       StateScanning,
		registry:    reg,
		initialDisk: dStats,
		spinner:     s,
		categories:  make([]CategoryItem, 0),
		cursor:      0,
		itemCursor:  0,
		cleanStatus: "Ejecutando limpieza segura de artefactos...",
		width:       80,
		height:      24,
	}
}

// Messages
type scanFinishedMsg struct {
	reports []plugins.PluginReport
	err     error
}

type cleanFinishedMsg struct {
	freedBytes int64
	finalDisk  *disk.DiskStats
	err        error
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startScanCmd(),
	)
}

func (m Model) startScanCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		reports, err := m.registry.ScanAll(ctx)
		return scanFinishedMsg{reports: reports, err: err}
	}
}

func (m Model) startCleanCmd() tea.Cmd {
	return func() tea.Msg {
		var totalFreed int64
		var cleanErrors []error

		for _, cat := range m.categories {
			plugin := m.registry.Find(cat.Report.PluginID)
			if plugin == nil {
				cleanErrors = append(cleanErrors, fmt.Errorf("plugin %q is no longer registered", cat.Report.PluginID))
				continue
			}

			var toClean []string
			for _, item := range cat.Report.Items {
				if cat.SelectedItems[item.ID] {
					toClean = append(toClean, item.ID)
				}
			}

			if len(toClean) == 0 {
				continue
			}

			freed, err := plugin.Clean(toClean)
			if err == nil {
				totalFreed += freed
			} else {
				totalFreed += freed
				cleanErrors = append(cleanErrors, fmt.Errorf("%s: %w", cat.Report.Title, err))
			}
		}

		finalStats, diskErr := disk.GetDiskUsage("/")
		if diskErr != nil {
			cleanErrors = append(cleanErrors, diskErr)
		}
		return cleanFinishedMsg{
			freedBytes: totalFreed,
			finalDisk:  finalStats,
			err:        errors.Join(cleanErrors...),
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scanFinishedMsg:
		if msg.err != nil && len(msg.reports) == 0 {
			m.err = msg.err
			m.state = StateError
			return m, nil
		}

		m.categories = make([]CategoryItem, 0)
		for _, rep := range msg.reports {
			if len(rep.Items) == 0 {
				continue
			}

			itemMap := make(map[string]bool)
			for _, it := range rep.Items {
				itemMap[it.ID] = true
			}

			m.categories = append(m.categories, CategoryItem{
				Report:        rep,
				Selected:      true,
				SelectedItems: itemMap,
			})
		}

		m.state = StateSelection
		m.cursor = 0
		if msg.err != nil {
			m.statusMessage = "El escaneo fue parcial: " + msg.err.Error()
		}
		return m, nil

	case cleanFinishedMsg:
		m.err = msg.err
		m.freedBytes = msg.freedBytes
		m.finalDisk = msg.finalDisk
		m.state = StateSummary
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

		switch m.state {
		case StateSelection:
			return m.updateSelection(msg)
		case StateDrillDown:
			return m.updateDrillDown(msg)
		case StateSummary:
			switch msg.String() {
			case "q", "enter", "esc":
				return m, tea.Quit
			}
		case StateError:
			switch msg.String() {
			case "q", "enter", "esc":
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m Model) updateSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "c" && m.confirmPending {
		m.confirmPending = false
		m.statusMessage = "Confirmación cancelada porque cambió la selección o navegación."
	}
	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.categories)-1 {
			m.cursor++
		}

	case " ":
		if len(m.categories) > 0 {
			cat := &m.categories[m.cursor]
			cat.Selected = !cat.Selected
			for _, it := range cat.Report.Items {
				cat.SelectedItems[it.ID] = cat.Selected
			}
		}

	case "a":
		allSelected := true
		for _, cat := range m.categories {
			if !cat.Selected {
				allSelected = false
				break
			}
		}
		newVal := !allSelected
		for i := range m.categories {
			m.categories[i].Selected = newVal
			for _, it := range m.categories[i].Report.Items {
				m.categories[i].SelectedItems[it.ID] = newVal
			}
		}

	case "d", "enter":
		if len(m.categories) > 0 {
			m.state = StateDrillDown
			m.itemCursor = 0
		}

	case "c":
		var totalSelected int64
		for _, cat := range m.categories {
			totalSelected += cat.SelectedBytes()
		}

		if totalSelected == 0 {
			m.statusMessage = "Selecciona al menos una categoría o elemento antes de limpiar."
			return m, nil
		}

		if !m.confirmPending {
			m.confirmPending = true
			m.statusMessage = "Presiona c nuevamente para confirmar la eliminación de los elementos seleccionados."
			return m, nil
		}
		m.confirmPending = false
		m.state = StateCleaning
		m.cleanStatus = "Ejecutando limpieza segura de artefactos y cachés..."
		return m, tea.Batch(m.spinner.Tick, m.startCleanCmd())
	}

	return m, nil
}

func (m Model) updateDrillDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.categories) == 0 {
		m.state = StateSelection
		return m, nil
	}

	cat := &m.categories[m.cursor]
	itemCount := len(cat.Report.Items)

	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "esc", "b":
		m.state = StateSelection
		return m, nil

	case "enter":
		m.state = StateSelection
		return m, nil

	case "up", "k":
		if m.itemCursor > 0 {
			m.itemCursor--
		}

	case "down", "j":
		if m.itemCursor < itemCount-1 {
			m.itemCursor++
		}

	case " ":
		if itemCount > 0 {
			currItem := cat.Report.Items[m.itemCursor]
			cat.SelectedItems[currItem.ID] = !cat.SelectedItems[currItem.ID]

			hasAny := false
			for _, sel := range cat.SelectedItems {
				if sel {
					hasAny = true
					break
				}
			}
			cat.Selected = hasAny
		}

	case "a":
		allSel := true
		for _, it := range cat.Report.Items {
			if !cat.SelectedItems[it.ID] {
				allSel = false
				break
			}
		}
		newVal := !allSel
		for _, it := range cat.Report.Items {
			cat.SelectedItems[it.ID] = newVal
		}
		cat.Selected = newVal
	}

	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case StateScanning:
		return m.viewScanning()
	case StateSelection:
		return m.viewSelection()
	case StateDrillDown:
		return m.viewDrillDown()
	case StateCleaning:
		return m.viewCleaning()
	case StateSummary:
		return m.viewSummary()
	case StateError:
		return fmt.Sprintf("\nError: %v\nPresiona 'q' para salir.\n", m.err)
	default:
		return ""
	}
}
