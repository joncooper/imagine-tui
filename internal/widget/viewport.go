package widget

import "fmt"

// viewport calculates which items to display in a scrollable view.
type viewport struct {
	offset int // first visible item index
}

// viewSlice describes the visible range of items.
type viewSlice struct {
	Start int // first visible item index (inclusive)
	End   int // last visible item index (exclusive)
	Above int // items hidden above
	Below int // items hidden below
}

// slice returns the range of items to render given the viewport height and
// a "follow" index (typically the selected row) that must remain visible.
// If height <= 0 or total == 0, returns the full range.
func (v *viewport) slice(total, height, follow int) viewSlice {
	if height <= 0 || total == 0 {
		return viewSlice{Start: 0, End: total}
	}
	if total <= height {
		v.offset = 0
		return viewSlice{Start: 0, End: total}
	}

	// Ensure follow index is within the visible window.
	if follow < v.offset {
		v.offset = follow
	}
	if follow >= v.offset+height {
		v.offset = follow - height + 1
	}

	// Clamp offset.
	if v.offset < 0 {
		v.offset = 0
	}
	if v.offset > total-height {
		v.offset = total - height
	}

	end := v.offset + height
	if end > total {
		end = total
	}

	return viewSlice{
		Start: v.offset,
		End:   end,
		Above: v.offset,
		Below: total - end,
	}
}

// scrollHint returns a string like "▲ 3 more" or "▼ 12 more", or empty.
func scrollHint(count int, up bool) string {
	if count <= 0 {
		return ""
	}
	arrow := "▼"
	if up {
		arrow = "▲"
	}
	return fmt.Sprintf("%s %d more", arrow, count)
}
