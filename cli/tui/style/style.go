package style

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Brand colors.
const (
	ColorGold   = "#E8C228"
	ColorDim    = "#666666"
	ColorBright = "#FFFFFF"
	ColorGreen  = "#04B575"
	ColorRed    = "#FF4672"
	ColorPurple = "#7B61FF"
)

// Reusable styles.
var (
	GoldStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold))
	BrightStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorBright))
	DimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
	TitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPurple))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGreen))
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed))

	BannerBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorGold)).
			Padding(1, 3)

	SummaryBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorGreen)).
			Padding(1, 3).
			MarginTop(1)

	ResultBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorGreen)).
			Padding(1, 3).
			MarginTop(1).
			MarginBottom(1)
)

const falconArtRaw = `
●●●●●●●●●●◐◐◐◐◐●●●●●●●●●●
●●●●●●●●◐◐◐◐◐◐◐◐◐◐●●●●●●●
●●●●●◐●●●●●●●●●●●◐◐◐●●●●●
●●●●●◐◐●●●●●●●●●●●●◐◐●●●●
●●◐●●●◐◐●●●●●●●●●●●●◐◐●●●
●●●◐●●●◐◐◐●●●●●●●●●●●◐◐●●
●●●◐◐◐●◐◐◐◐●●●●●●●●●●●◐●●
●●●●◐◐◐◐◐◐◐◐◐●●●●●●●●●◐◐●
●◐◐●●◐◐◐◐◐◐◐◐◐◐●●●●●●●●◐●
●●◐◐◐●◐◐◐◐◐◐◐◐◐◐◐◐●●●●●◐●
●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●●◐●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●◐●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●
●◐◐●●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●
●●◐●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●
●●◐◐●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●●
●●●◐●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●●
●●●◐◐●●●●●◐◐◐◐◐◐◐◐◐◐◐◐●●●
●●●●◐◐◐●●●●●●●●●●●◐◐◐●●●●
●●●●●◐◐◐●●●●●●●●●◐◐◐●●●●●
●●●●●●●◐◐◐◐◐◐◐◐◐◐◐●●●●●●●
●●●●●●●●●●◐◐◐◐◐●●●●●●●●●●`

func renderFalconLogo() string {
	raw := strings.Trim(falconArtRaw, "\n")
	lines := strings.Split(raw, "\n")

	packed := make([]string, 0, (len(lines)+1)/2)
	for i := 0; i < len(lines); i += 2 {
		upper := []rune(lines[i])
		var lower []rune
		if i+1 < len(lines) {
			lower = []rune(lines[i+1])
		}

		maxLen := len(upper)
		if len(lower) > maxLen {
			maxLen = len(lower)
		}

		row := make([]rune, maxLen)
		for j := 0; j < maxLen; j++ {
			upOn := j < len(upper) && upper[j] == '◐'
			dnOn := j < len(lower) && lower[j] == '◐'

			switch {
			case upOn && dnOn:
				row[j] = '█'
			case upOn:
				row[j] = '▀'
			case dnOn:
				row[j] = '▄'
			default:
				row[j] = ' '
			}
		}

		packed = append(packed, string(row))
	}

	return GoldStyle.Render(strings.Join(packed, "\n"))
}

const wordmarkArt = `
██████  ███████ ██████  ███████  ██████  ██████  ██ ███    ██ ███████
██   ██ ██      ██   ██ ██      ██       ██   ██ ██ ████   ██ ██
██████  █████   ██████  █████   ██   ███ ██████  ██ ██ ██  ██ █████
██      ██      ██   ██ ██      ██    ██ ██   ██ ██ ██  ██ ██ ██
██      ███████ ██   ██ ███████  ██████  ██   ██ ██ ██   ████ ███████`

// RenderBanner produces the branded header with logo and wordmark.
func RenderBanner() string {
	falcon := renderFalconLogo()
	wordmark := BrightStyle.Render(strings.Trim(wordmarkArt, "\n"))
	subtitle := DimStyle.Render("                     DIGITAL SERVICES")

	text := lipgloss.JoinVertical(lipgloss.Left, wordmark, subtitle)
	logo := lipgloss.JoinHorizontal(lipgloss.Center, falcon, "    ", text)
	return BannerBox.Render(logo)
}

// CenterContent vertically positions content in the upper portion of the terminal.
func CenterContent(content string, height int) string {
	if height > 0 {
		contentLines := strings.Count(content, "\n") + 1
		topPad := (height - contentLines) / 4
		if topPad > 1 {
			content = strings.Repeat("\n", topPad) + content
		}
	}
	return content
}
