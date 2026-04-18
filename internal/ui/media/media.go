package media

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	apptg "github.com/dxlongnh/telegram-tui/internal/tg"
)

// Model is the media viewer overlay sub-model.
type Model struct {
	info    *apptg.MediaInfo
	loading bool
	path    string
	err     error
	width   int
	height  int
}

// New creates an inactive media viewer.
func New(width, height int) Model {
	return Model{width: width, height: height}
}

// SetSize updates terminal dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Open shows the media viewer for the given media info, in loading state.
func (m *Model) Open(info *apptg.MediaInfo) {
	m.info = info
	m.loading = true
	m.path = ""
	m.err = nil
}

// SetReady marks the download as complete with the local file path.
func (m *Model) SetReady(path string) {
	m.loading = false
	m.path = path
}

// SetError records a download error.
func (m *Model) SetError(err error) {
	m.loading = false
	m.err = err
}

// Active reports whether the viewer is shown.
func (m Model) Active() bool { return m.info != nil }

// Path returns the local download path (empty if not yet ready).
func (m Model) Path() string { return m.path }

// View renders the media overlay.
func (m Model) View() string {
	if m.info == nil {
		return ""
	}

	var sb strings.Builder

	if m.loading {
		sb.WriteString(titleStyle.Render("Downloading media…"))
	} else if m.err != nil {
		sb.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	} else {
		sb.WriteString(titleStyle.Render("Media Viewer"))
		sb.WriteString("\n\n")
		sb.WriteString(labelStyle.Render("Type:     ") + typeStr(m.info.Type))
		sb.WriteString("\n")
		if m.info.FileName != "" {
			sb.WriteString(labelStyle.Render("File:     ") + m.info.FileName)
			sb.WriteString("\n")
		}
		if m.info.Size > 0 {
			sb.WriteString(labelStyle.Render("Size:     ") + humanSize(m.info.Size))
			sb.WriteString("\n")
		}
		if m.info.MimeType != "" {
			sb.WriteString(labelStyle.Render("MIME:     ") + m.info.MimeType)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(hintStyle.Render("[Enter] Open in system viewer  [Esc] Close"))
	}

	inner := sb.String()
	box := boxStyle.Render(inner)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func typeStr(t apptg.MediaType) string {
	switch t {
	case apptg.MediaPhoto:
		return "📷 Photo"
	case apptg.MediaVideo:
		return "🎥 Video"
	case apptg.MediaAudio:
		return "🎵 Audio"
	case apptg.MediaDocument:
		return "📎 Document"
	default:
		return "Media"
	}
}

func humanSize(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

var (
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#61AFEF")).
			Background(lipgloss.Color("#21252B")).
			Padding(1, 3)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#61AFEF"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ABB2BF")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E06C75"))
)
