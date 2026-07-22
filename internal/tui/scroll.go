package tui

// chromeLines returns non-list lines consumed by View chrome.
// base: title line, blank after title, blank after list, help line.
// +2 when an input box is shown; +1 when status or error is shown.
func chromeLines(inputActive, hasStatus bool) int {
	n := 4
	if inputActive {
		n += 2
	}
	if hasStatus {
		n++
	}
	return n
}

// listVisible returns how many list rows fit. Always ≥ 1.
func listVisible(height, chrome int) int {
	v := height - chrome
	if v < 1 {
		return 1
	}
	return v
}

// ensureOffset returns a new scroll offset so cursor is inside the window
// [offset, offset+visible). Also clamps offset into [0, max(0, total-visible)].
func ensureOffset(offset, cursor, visible, total int) int {
	if total <= 0 || visible <= 0 {
		return 0
	}
	if visible >= total {
		return 0
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+visible {
		offset = cursor - visible + 1
	}
	maxOff := total - visible
	if offset < 0 {
		offset = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	return offset
}
