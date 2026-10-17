package display

import (
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/tcell/v2"
)

type completionPopupItem struct {
	label  string
	kind   string
	detail string
}

// completionPopup is a lightweight overlay rendered using Micro's screen and
// colorscheme systems. It owns no pane or buffer state; BufWindow supplies the
// viewport-relative geometry and the currently selected items.
type completionPopup struct {
	x, y, width, rows int
	first, selected   int
	items             []completionPopupItem
}

func parseCompletionPopupItems(suggestions []string) []completionPopupItem {
	items := make([]completionPopupItem, 0, len(suggestions))
	for _, suggestion := range suggestions {
		label, rest, hasMetadata := strings.Cut(suggestion, "\t")
		item := completionPopupItem{label: label}
		if hasMetadata {
			var hasDetail bool
			item.kind, item.detail, hasDetail = strings.Cut(rest, "\t")
			if !hasDetail {
				item.kind = rest
			}
		}
		items = append(items, item)
	}
	return items
}

func completionStyle(names []string, fallback tcell.Style) tcell.Style {
	for _, name := range names {
		if style, ok := config.Colorscheme[name]; ok {
			return style
		}
	}
	return fallback
}

func completionTextWidth(text string) int {
	return runewidth.StringWidth(text)
}

// fitCompletionText truncates by terminal cell width rather than rune count.
func fitCompletionText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if completionTextWidth(text) <= maxWidth {
		return text
	}
	if maxWidth == 1 {
		return "…"
	}

	var out strings.Builder
	width := 0
	for _, r := range text {
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 1
		}
		if width+rw > maxWidth-1 {
			break
		}
		out.WriteRune(r)
		width += rw
	}
	out.WriteRune('…')
	return out.String()
}

func (p completionPopup) display() {
	if p.width < 8 || p.rows < 1 || len(p.items) == 0 {
		return
	}
	screenWidth, screenHeight := screen.Screen.Size()

	panelStyle := completionStyle(
		[]string{"autocomplete.popup", "message"},
		config.DefStyle,
	)
	borderStyle := completionStyle(
		[]string{"autocomplete.border", "divider"},
		config.DefStyle.Reverse(true),
	)
	selectedStyle := completionStyle(
		[]string{"autocomplete.selected", "selection", "statusline.suggestions"},
		config.DefStyle.Reverse(true),
	)
	kindStyle := completionStyle(
		[]string{"autocomplete.kind", "type", "identifier"},
		config.DefStyle,
	)
	detailStyle := completionStyle(
		[]string{"autocomplete.detail", "comment"},
		panelStyle,
	)

	put := func(x, y int, r rune, style tcell.Style) {
		if x >= 0 && x < screenWidth && y >= 0 && y < screenHeight {
			screen.SetContent(x, y, r, nil, style)
		}
	}
	fill := func(x, y, width int, style tcell.Style) {
		for i := 0; i < width; i++ {
			put(x+i, y, ' ', style)
		}
	}
	drawText := func(x, y int, text string, maxWidth int, style tcell.Style) int {
		used := 0
		for _, r := range text {
			rw := runewidth.RuneWidth(r)
			if rw < 0 {
				rw = 1
			}
			if rw == 0 {
				put(x+used-1, y, r, style)
				continue
			}
			if used+rw > maxWidth {
				break
			}
			put(x+used, y, r, style)
			for j := 1; j < rw; j++ {
				put(x+used+j, y, ' ', style)
			}
			used += rw
		}
		return used
	}

	height := p.rows + 2
	// Opaque interior prevents text under the popup from bleeding through.
	for row := 1; row < height-1; row++ {
		fill(p.x+1, p.y+row, p.width-2, panelStyle)
	}

	// Border tint comes from the active colorscheme, not a hard-coded RGB value.
	put(p.x, p.y, '╭', borderStyle)
	put(p.x+p.width-1, p.y, '╮', borderStyle)
	put(p.x, p.y+height-1, '╰', borderStyle)
	put(p.x+p.width-1, p.y+height-1, '╯', borderStyle)
	for col := 1; col < p.width-1; col++ {
		put(p.x+col, p.y, '─', borderStyle)
		put(p.x+col, p.y+height-1, '─', borderStyle)
	}
	for row := 1; row < height-1; row++ {
		put(p.x, p.y+row, '│', borderStyle)
		put(p.x+p.width-1, p.y+row, '│', borderStyle)
	}

	for row := 0; row < p.rows; row++ {
		index := p.first + row
		if index < 0 || index >= len(p.items) {
			continue
		}
		item := p.items[index]
		y := p.y + row + 1
		rowStyle := panelStyle
		if index == p.selected {
			rowStyle = selectedStyle
		}
		fill(p.x+1, y, p.width-2, rowStyle)

		innerWidth := p.width - 4
		kindWidth := completionTextWidth(item.kind)
		maxRightWidth := innerWidth / 2
		if kindWidth > maxRightWidth {
			item.kind = fitCompletionText(item.kind, maxRightWidth)
			kindWidth = completionTextWidth(item.kind)
		}
		maxDetailWidth := maxRightWidth - kindWidth
		if kindWidth > 0 && item.detail != "" {
			maxDetailWidth--
		}
		if maxDetailWidth < 0 {
			maxDetailWidth = 0
		}
		item.detail = fitCompletionText(item.detail, maxDetailWidth)
		detailWidth := completionTextWidth(item.detail)
		rightWidth := kindWidth + detailWidth
		if kindWidth > 0 && detailWidth > 0 {
			rightWidth++
		}

		labelWidth := innerWidth - rightWidth
		if rightWidth > 0 {
			labelWidth--
		}
		if labelWidth < 1 {
			labelWidth = 1
		}

		label := fitCompletionText(item.label, labelWidth)
		labelStyle := rowStyle
		metaStyle := detailStyle
		glyphStyle := kindStyle
		if index == p.selected {
			metaStyle = selectedStyle
			glyphStyle = selectedStyle
		}

		x := p.x + 2
		drawText(x, y, label, labelWidth, labelStyle)

		rightX := p.x + p.width - 2
		if kindWidth > 0 {
			rightX -= kindWidth
			drawText(rightX, y, item.kind, kindWidth, glyphStyle)
		}
		if detailWidth > 0 {
			rightX -= detailWidth
			if kindWidth > 0 {
				rightX--
			}
			drawText(rightX, y, item.detail, detailWidth, metaStyle)
		}
	}
}
