package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var Version = "dev"

var (
	// Palette Colors
	ColorCyan    = lipgloss.Color("#00ADD8") // Primary Brand Cyan/Turquoise
	ColorGreen   = lipgloss.Color("#2ECC71") // Success / Reclaimed Green
	ColorYellow  = lipgloss.Color("#F1C40F") // Safety / Warning Accent
	ColorRed     = lipgloss.Color("#E74C3C") // Used Space Indicator
	ColorGrey    = lipgloss.Color("#7F8C8D") // Subtle Metadata Grey
	ColorLight   = lipgloss.Color("#F8F9FA") // Crisp Off-White
	ColorDarkBg  = lipgloss.Color("#1E272E") // Dark Charcoal
	ColorSurface = lipgloss.Color("#2C3A47") // Container Surface

	// Header Styles
	BannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCyan)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	AuthorSponsorStyle = lipgloss.NewStyle().
				Foreground(ColorGrey).
				Italic(true)

	// Container & Card Styles
	BoxCard = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#485460")).
		Padding(0, 1).
		MarginBottom(1)

	FocusedCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorCyan).
			Padding(0, 1).
			MarginBottom(1)

	// Disk Bar Styles
	BarFilledStyle = lipgloss.NewStyle().
			Foreground(ColorRed)

	BarEmptyStyle = lipgloss.NewStyle().
			Foreground(ColorGreen)

	// Category List & Drill-down Styles
	CursorStyle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	CheckboxChecked = lipgloss.NewStyle().
			Foreground(ColorGreen).
			Bold(true)

	CheckboxUnchecked = lipgloss.NewStyle().
				Foreground(ColorGrey)

	CategoryTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorLight)

	CategoryTitleFocused = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorCyan)

	SizeTagStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorYellow)

	SafetyBadge = lipgloss.NewStyle().
			Foreground(ColorYellow).
			Bold(true)

	SafetyText = lipgloss.NewStyle().
			Foreground(ColorGrey).
			Italic(true)

	ItemPathStyle = lipgloss.NewStyle().
			Foreground(ColorLight)

	ItemAgeStyle = lipgloss.NewStyle().
			Foreground(ColorGrey)

	// Navigation & Keys Footer
	KeyStyle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	KeyDescStyle = lipgloss.NewStyle().
			Foreground(ColorGrey)

	StatusBar = lipgloss.NewStyle().
			Foreground(ColorLight).
			Background(ColorSurface).
			Padding(0, 1)

	// Summary Styles
	SuccessTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorGreen).
			Padding(0, 1)

	RecoveredStat = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorGreen)

	SponsorCallout = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Italic(true)
)
