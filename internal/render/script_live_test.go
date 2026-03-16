package render

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	"github.com/joncooper/imagine-tui/internal/widget"
)

func TestRenderRunsMountAndComputedScriptsOnDOMChange(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{
					"id":    "name",
					"type":  "input",
					"props": map[string]any{"value": "alpha"},
				},
				map[string]any{
					"id":       "computed",
					"type":     "text",
					"computed": map[string]any{"text": "return $('name').value.toUpperCase()"},
				},
				map[string]any{
					"id":      "mounted",
					"type":    "text",
					"props":   map[string]any{"text": "pending"},
					"scripts": map[string]any{"on_mount": "$.text = 'mounted'"},
				},
			},
		},
	})

	newM, _ := m.Update(DOMChangedMsg{Ctx: context.Background()})
	model := newM.(Model)
	view := model.View()
	if !strings.Contains(view, "ALPHA") {
		t.Errorf("computed prop not rendered, got:\n%s", view)
	}
	if !strings.Contains(view, "mounted") {
		t.Errorf("mount hook not rendered, got:\n%s", view)
	}
}

func TestRenderRunsOnChangeScriptsAndQueuesAgentEvents(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{
					"id":    "status",
					"type":  "text",
					"props": map[string]any{"text": "idle"},
				},
				map[string]any{
					"id":   "search",
					"type": "input",
					"scripts": map[string]any{
						"on_change": "emit('local', [{op: 'update', id: 'status', props: {text: 'query:' + $.value}}]); emit('agent', {action: 'filter', query: $.value})",
					},
				},
			},
		},
	})

	newM, _ := m.Update(DOMChangedMsg{Ctx: context.Background()})
	model := newM.(Model)
	model.focusedID = "search"

	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model2 := newM2.(Model)
	view := model2.View()
	if !strings.Contains(view, "query:x") {
		t.Errorf("local patch from script not rendered, got:\n%s", view)
	}

	result := callTool(t, srv, "await_event", map[string]any{"timeout_ms": 50})
	text := resultText(t, result)
	if !strings.Contains(text, "\"action\":\"filter\"") {
		t.Errorf("expected agent event payload, got: %s", text)
	}
	if !strings.Contains(text, "\"source\":\"search\"") {
		t.Errorf("expected script event source, got: %s", text)
	}
}

func TestRenderRunsOnKeyScriptsWithoutButtonClickFallback(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{
					"id":    "status",
					"type":  "text",
					"props": map[string]any{"text": "idle"},
				},
				map[string]any{
					"id":    "action",
					"type":  "button",
					"props": map[string]any{"label": "Launch"},
					"scripts": map[string]any{
						"on_key": "$.props.label = 'Pressed'; emit('local', [{op: 'update', id: 'status', props: {text: 'armed'}}])",
					},
				},
			},
		},
	})

	newM, _ := m.Update(DOMChangedMsg{Ctx: context.Background()})
	model := newM.(Model)
	model.focusedID = "action"

	newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model2 := newM2.(Model)
	view := model2.View()
	if !strings.Contains(view, "Pressed") || !strings.Contains(view, "armed") {
		t.Errorf("on_key script did not update view, got:\n%s", view)
	}

	result := callTool(t, srv, "await_event", map[string]any{"timeout_ms": 10})
	text := resultText(t, result)
	if !strings.Contains(text, "timeout") {
		t.Errorf("expected no fallback click event, got: %s", text)
	}
}

func TestRenderRunsTimerCallbacksViaTick(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":      "root",
			"type":    "container",
			"scripts": map[string]any{"on_mount": "setTimeout(function() { $('status').text = 'done'; }, 1)"},
			"children": []any{
				map[string]any{
					"id":    "status",
					"type":  "text",
					"props": map[string]any{"text": "idle"},
				},
			},
		},
	})

	newM, cmd := m.Update(DOMChangedMsg{Ctx: context.Background()})
	if cmd == nil {
		t.Fatal("expected DOMChangedMsg to schedule a timer tick")
	}

	timerMsg := cmd()
	if _, ok := timerMsg.(scriptTimerMsg); !ok {
		t.Fatalf("expected scriptTimerMsg, got %T", timerMsg)
	}

	model := newM.(Model)
	newM2, _ := model.Update(timerMsg)
	view := newM2.(Model).View()
	if !strings.Contains(view, "done") {
		t.Errorf("timer callback did not update view, got:\n%s", view)
	}
}

func TestRenderCancelsTimersWhenOwnerNodeRemoved(t *testing.T) {
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	m := NewModel(srv, widget.DefaultRegistry())
	m.width = 80
	m.height = 24

	callTool(t, srv, "replace", map[string]any{
		"tree": map[string]any{
			"id":   "root",
			"type": "container",
			"children": []any{
				map[string]any{
					"id":    "status",
					"type":  "text",
					"props": map[string]any{"text": "idle"},
				},
				map[string]any{
					"id":      "countdown",
					"type":    "text",
					"scripts": map[string]any{"on_mount": "setTimeout(function() { $('status').text = 'boom'; }, 10)"},
				},
			},
		},
	})

	newM, cmd := m.Update(DOMChangedMsg{Ctx: context.Background()})
	if cmd == nil {
		t.Fatal("expected DOMChangedMsg to schedule a timer tick")
	}
	model := newM.(Model)

	callTool(t, srv, "patch", map[string]any{
		"ops": []map[string]any{
			{
				"op": "remove",
				"id": "countdown",
			},
		},
	})

	newM2, _ := model.Update(DOMChangedMsg{Ctx: context.Background()})
	model2 := newM2.(Model)

	timerMsg := cmd()
	newM3, _ := model2.Update(timerMsg)
	view := newM3.(Model).View()
	if strings.Contains(view, "boom") {
		t.Errorf("removed node timer should not have fired, got:\n%s", view)
	}
	if !strings.Contains(view, "idle") {
		t.Errorf("expected status to remain idle, got:\n%s", view)
	}
}
