package tui

import (
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderBox renders a btop-style rounded box with optional titles embedded in
// the top border (left- and right-aligned). The content string is styled as-is;
// lines are padded/truncated to the inner width, and any extra lines are
// dropped so the box height is always exactly `height`.
func renderBox(leftTitle, rightTitle, content string, boxWidth, height int, borderColor lp.Color) string {
	const (
		topLeft     = "╭"
		topRight    = "╮"
		bottomLeft  = "╰"
		bottomRight = "╯"
		horizontal  = "─"
		vertical    = "│"
	)

	if boxWidth < boxBorderFrameWidth {
		boxWidth = boxBorderFrameWidth
	}
	if height < boxBorderFrameHeight {
		height = boxBorderFrameHeight
	}

	innerWidth := boxWidth - boxBorderFrameWidth
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Truncate titles so they can never widen the top border beyond the box.
	leftW := width(leftTitle)
	rightW := width(rightTitle)
	const minDashes = 1
	maxTitleSpace := innerWidth - minDashes
	if maxTitleSpace <= 0 {
		leftTitle, leftW = "", 0
		rightTitle, rightW = "", 0
	} else if leftW+rightW > maxTitleSpace {
		half := maxTitleSpace / 2
		if leftW > half {
			leftTitle = truncate(leftTitle, half)
			leftW = width(leftTitle)
		}
		if leftW+rightW > maxTitleSpace {
			remaining := maxTitleSpace - leftW
			if remaining <= 0 {
				rightTitle, rightW = "", 0
			} else {
				rightTitle = truncate(rightTitle, remaining)
				rightW = width(rightTitle)
			}
		}
	}

	border := lp.NewStyle().Foreground(borderColor)

	var topBorder string
	switch {
	case leftTitle != "" && rightTitle != "":
		remaining := innerWidth - leftW - rightW - 1
		if remaining < 0 {
			remaining = 0
		}
		topBorder = border.Render(topLeft+horizontal) +
			leftTitle +
			border.Render(strings.Repeat(horizontal, remaining)) +
			rightTitle +
			border.Render(topRight)
	case leftTitle != "":
		remaining := innerWidth - leftW - 1
		if remaining < 0 {
			remaining = 0
		}
		topBorder = border.Render(topLeft+horizontal) +
			leftTitle +
			border.Render(strings.Repeat(horizontal, remaining)+topRight)
	case rightTitle != "":
		remaining := innerWidth - rightW - 1
		if remaining < 0 {
			remaining = 0
		}
		topBorder = border.Render(topLeft+strings.Repeat(horizontal, remaining)) +
			rightTitle +
			border.Render(horizontal+topRight)
	default:
		topBorder = border.Render(topLeft + strings.Repeat(horizontal, innerWidth) + topRight)
	}

	bottomBorder := border.Render(bottomLeft + strings.Repeat(horizontal, innerWidth) + bottomRight)

	innerHeight := height - boxBorderFrameHeight
	lines := strings.Split(content, "\n")

	rendered := make([]string, 0, height)
	rendered = append(rendered, topBorder)
	for i := 0; i < innerHeight; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		lw := width(line)
		switch {
		case lw < innerWidth:
			line += strings.Repeat(" ", innerWidth-lw)
		case lw > innerWidth:
			line = truncate(line, innerWidth)
			if pad := innerWidth - width(line); pad > 0 {
				line += strings.Repeat(" ", pad)
			}
		}
		rendered = append(rendered, border.Render(vertical)+line+border.Render(vertical))
	}
	rendered = append(rendered, bottomBorder)

	return strings.Join(rendered, "\n")
}

// joinColumns lays out two multi-line blocks side by side, separated by gap,
// padding the shorter block with blank lines. Both blocks are expected to be the
// same width and height (as produced by renderBox).
func joinColumns(left, right, gap string) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	n := len(leftLines)
	if len(rightLines) > n {
		n = len(rightLines)
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		var l, r string
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		out[i] = l + gap + r
	}
	return strings.Join(out, "\n")
}
