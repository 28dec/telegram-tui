package style

import (
	"encoding/binary"
	"hash/fnv"
	"image/color"

	"charm.land/lipgloss/v2"
)

// palette of distinct terminal colors for username coloring.
var palette = []color.Color{
	lipgloss.Color("#E06C75"), // red
	lipgloss.Color("#98C379"), // green
	lipgloss.Color("#E5C07B"), // yellow
	lipgloss.Color("#61AFEF"), // blue
	lipgloss.Color("#C678DD"), // purple
	lipgloss.Color("#56B6C2"), // cyan
	lipgloss.Color("#D19A66"), // orange
	lipgloss.Color("#BE5046"), // dark red
	lipgloss.Color("#7EC8A4"), // mint
	lipgloss.Color("#F0A500"), // gold
	lipgloss.Color("#BB9AF7"), // lavender
	lipgloss.Color("#73DACA"), // teal
	lipgloss.Color("#FF9E64"), // peach
	lipgloss.Color("#9ECE6A"), // lime
	lipgloss.Color("#2AC3DE"), // sky
	lipgloss.Color("#F7768E"), // pink
}

// ColorForUser returns a lipgloss style with a deterministic foreground color
// based on the user's ID.
func ColorForUser(userID int64) lipgloss.Style {
	h := fnv.New32a()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(userID))
	h.Write(b[:])
	idx := h.Sum32() % uint32(len(palette))
	return lipgloss.NewStyle().Foreground(palette[idx])
}
