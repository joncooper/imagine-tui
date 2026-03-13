package widget

import "testing"

func TestViewport_FullyVisible(t *testing.T) {
	v := &viewport{}
	s := v.slice(5, 10, 0)
	if s.Start != 0 || s.End != 5 {
		t.Errorf("got [%d:%d], want [0:5]", s.Start, s.End)
	}
	if s.Above != 0 || s.Below != 0 {
		t.Errorf("above=%d below=%d, want 0,0", s.Above, s.Below)
	}
}

func TestViewport_ScrollDown(t *testing.T) {
	v := &viewport{}
	// 20 items, viewport of 5, following item 7
	s := v.slice(20, 5, 7)
	if s.Start > 7 || s.End <= 7 {
		t.Errorf("follow=7 not in [%d:%d]", s.Start, s.End)
	}
	if s.End-s.Start != 5 {
		t.Errorf("visible=%d, want 5", s.End-s.Start)
	}
	if s.Above != s.Start {
		t.Errorf("above=%d, want %d", s.Above, s.Start)
	}
	if s.Below != 20-s.End {
		t.Errorf("below=%d, want %d", s.Below, 20-s.End)
	}
}

func TestViewport_ScrollUp(t *testing.T) {
	v := &viewport{offset: 10}
	// 20 items, viewport of 5, follow item 3 (above current offset)
	s := v.slice(20, 5, 3)
	if s.Start != 3 {
		t.Errorf("start=%d, want 3", s.Start)
	}
	if s.End != 8 {
		t.Errorf("end=%d, want 8", s.End)
	}
}

func TestViewport_FollowAtEnd(t *testing.T) {
	v := &viewport{}
	s := v.slice(20, 5, 19)
	if s.End != 20 {
		t.Errorf("end=%d, want 20", s.End)
	}
	if s.Start != 15 {
		t.Errorf("start=%d, want 15", s.Start)
	}
}

func TestViewport_ZeroHeight(t *testing.T) {
	v := &viewport{}
	s := v.slice(10, 0, 0)
	if s.Start != 0 || s.End != 10 {
		t.Errorf("got [%d:%d], want [0:10]", s.Start, s.End)
	}
}

func TestViewport_EmptyItems(t *testing.T) {
	v := &viewport{}
	s := v.slice(0, 5, 0)
	if s.Start != 0 || s.End != 0 {
		t.Errorf("got [%d:%d], want [0:0]", s.Start, s.End)
	}
}

func TestViewport_Persistence(t *testing.T) {
	v := &viewport{}
	// Scroll to item 15
	v.slice(20, 5, 15)
	// Now follow item 14 — should not scroll since 14 is still visible
	s := v.slice(20, 5, 14)
	if s.Start > 14 || s.End <= 14 {
		t.Errorf("item 14 not visible in [%d:%d]", s.Start, s.End)
	}
}

func TestScrollHint(t *testing.T) {
	tests := []struct {
		count int
		up    bool
		want  string
	}{
		{0, true, ""},
		{3, true, "▲ 3 more"},
		{12, false, "▼ 12 more"},
	}
	for _, tt := range tests {
		got := scrollHint(tt.count, tt.up)
		if got != tt.want {
			t.Errorf("scrollHint(%d, %v) = %q, want %q", tt.count, tt.up, got, tt.want)
		}
	}
}
