package render

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	"github.com/joncooper/imagine-tui/internal/widget"
)

// setupPipeline creates a server + model with logger, simulates WindowSizeMsg,
// and returns the model ready for rendering. This mirrors the real BubbleTea
// startup sequence.
func setupPipeline(t *testing.T) (Model, *imcp.Server) {
	t.Helper()
	srv, err := imcp.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Shutdown)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv.SetLogger(logger)

	m := NewModel(srv, widget.DefaultRegistry())
	m.SetLogger(logger)

	// Simulate BubbleTea startup: WindowSizeMsg comes first.
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return newM.(Model), srv
}

// replaceTree calls the MCP replace tool and sends DOMChangedMsg to the model.
func replaceTree(t *testing.T, m Model, srv *imcp.Server, tree map[string]any) Model {
	t.Helper()
	result, err := srv.CallTool(context.Background(), "replace", map[string]any{"tree": tree})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if result.IsError {
		t.Fatalf("replace returned error: %v", result.Content)
	}
	newM, _ := m.Update(DOMChangedMsg{Ctx: context.Background()})
	return newM.(Model)
}

func patchTree(t *testing.T, m Model, srv *imcp.Server, ops []map[string]any) Model {
	t.Helper()
	result, err := srv.CallTool(context.Background(), "patch", map[string]any{"ops": ops})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if result.IsError {
		t.Fatalf("patch returned error: %v", result.Content)
	}
	newM, _ := m.Update(DOMChangedMsg{Ctx: context.Background()})
	return newM.(Model)
}

func TestRenderPipeline(t *testing.T) {
	t.Run("hello_world_text_prop", func(t *testing.T) {
		m, srv := setupPipeline(t)
		m = replaceTree(t, m, srv, map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{"id": "msg", "type": "text", "props": map[string]any{"text": "Hello World"}},
			},
		})

		view := m.View()
		if !strings.Contains(view, "Hello World") {
			t.Errorf("expected 'Hello World' in view, got:\n%s", view)
		}
	})

	t.Run("hello_world_content_prop", func(t *testing.T) {
		m, srv := setupPipeline(t)
		m = replaceTree(t, m, srv, map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{"id": "msg", "type": "text", "props": map[string]any{"content": "Hello Content"}},
			},
		})

		view := m.View()
		if !strings.Contains(view, "Hello Content") {
			t.Errorf("expected 'Hello Content' in view, got:\n%s", view)
		}
	})

	t.Run("button_renders_label", func(t *testing.T) {
		m, srv := setupPipeline(t)
		m = replaceTree(t, m, srv, map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{"id": "btn", "type": "button", "props": map[string]any{"label": "Click Me"}},
			},
		})

		view := m.View()
		if !strings.Contains(view, "Click Me") {
			t.Errorf("expected 'Click Me' in view, got:\n%s", view)
		}
	})

	t.Run("full_layout_with_multiple_widgets", func(t *testing.T) {
		m, srv := setupPipeline(t)
		m = replaceTree(t, m, srv, map[string]any{
			"id":    "root",
			"type":  "container",
			"props": map[string]any{"direction": "vertical"},
			"children": []map[string]any{
				{"id": "header", "type": "text", "props": map[string]any{"content": "Dashboard"}},
				{
					"id":    "body",
					"type":  "container",
					"props": map[string]any{"direction": "horizontal"},
					"children": []map[string]any{
						{"id": "btn-a", "type": "button", "props": map[string]any{"label": "Action A"}},
						{"id": "btn-b", "type": "button", "props": map[string]any{"label": "Action B"}},
					},
				},
				{"id": "search", "type": "input", "props": map[string]any{"placeholder": "Search..."}},
				{"id": "footer", "type": "text", "props": map[string]any{"content": "Status: OK"}},
			},
		})

		view := m.View()
		for _, expected := range []string{"Dashboard", "Action A", "Action B", "Search...", "Status: OK"} {
			if !strings.Contains(view, expected) {
				t.Errorf("expected %q in view, got:\n%s", expected, view)
			}
		}
	})

	t.Run("replace_then_patch", func(t *testing.T) {
		m, srv := setupPipeline(t)
		m = replaceTree(t, m, srv, map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{"id": "msg", "type": "text", "props": map[string]any{"content": "Before"}},
			},
		})

		view := m.View()
		if !strings.Contains(view, "Before") {
			t.Fatalf("expected 'Before' in view, got:\n%s", view)
		}

		m = patchTree(t, m, srv, []map[string]any{
			{"op": "update", "id": "msg", "props": map[string]any{"content": "After"}},
		})

		view = m.View()
		if !strings.Contains(view, "After") {
			t.Errorf("expected 'After' in view after patch, got:\n%s", view)
		}
		if strings.Contains(view, "Before") {
			t.Errorf("should not contain 'Before' after patch, got:\n%s", view)
		}
	})

	t.Run("zero_dimensions_then_resize", func(t *testing.T) {
		srv, err := imcp.NewServer()
		if err != nil {
			t.Fatal(err)
		}
		defer srv.Shutdown()

		m := NewModel(srv, widget.DefaultRegistry())
		// Do NOT send WindowSizeMsg — dimensions are 0.

		// Replace tree.
		_, _ = srv.CallTool(context.Background(), "replace", map[string]any{
			"tree": map[string]any{
				"id":   "root",
				"type": "container",
				"children": []map[string]any{
					{"id": "msg", "type": "text", "props": map[string]any{"content": "Waiting"}},
				},
			},
		})
		newM, _ := m.Update(DOMChangedMsg{Ctx: context.Background()})
		m = newM.(Model)

		// View should be empty because width=0, height=0.
		if view := m.View(); view != "" {
			t.Errorf("expected empty view with zero dimensions, got:\n%s", view)
		}

		// Now resize — the content should appear.
		newM, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m = newM.(Model)

		view := m.View()
		if !strings.Contains(view, "Waiting") {
			t.Errorf("expected 'Waiting' after resize, got:\n%s", view)
		}
	})

	t.Run("progress_widget_renders_between_siblings", func(t *testing.T) {
		m, srv := setupPipeline(t)
		m = replaceTree(t, m, srv, map[string]any{
			"id":   "root",
			"type": "container",
			"children": []map[string]any{
				{"id": "before", "type": "text", "props": map[string]any{"content": "Visible Before"}},
				{"id": "progress", "type": "progress", "props": map[string]any{"label": "Loading", "value": 50}},
				{"id": "after", "type": "text", "props": map[string]any{"content": "Visible After"}},
			},
		})

		view := m.View()
		if !strings.Contains(view, "Visible Before") {
			t.Errorf("expected 'Visible Before' with progress widget present, got:\n%s", view)
		}
		if !strings.Contains(view, "Visible After") {
			t.Errorf("expected 'Visible After' with progress widget present, got:\n%s", view)
		}
		if !strings.Contains(view, "Loading") {
			t.Errorf("expected progress widget label in output, got:\n%s", view)
		}
	})

	t.Run("template_expansion_with_set_items", func(t *testing.T) {
		m, srv := setupPipeline(t)

		// Use layout to define structure with item_template.
		result, err := srv.CallTool(context.Background(), "layout", map[string]any{
			"tree": map[string]any{
				"id":    "root",
				"type":  "container",
				"props": map[string]any{"direction": "vertical"},
				"children": []map[string]any{
					{"id": "header", "type": "text", "props": map[string]any{"content": "Log Viewer"}},
					{
						"id":   "log-list",
						"type": "container",
						"props": map[string]any{
							"direction": "vertical",
							"item_template": map[string]any{
								"type":  "container",
								"props": map[string]any{"direction": "row"},
								"children": []any{
									map[string]any{"type": "text", "props": map[string]any{"content": "[{{level}}]", "width": 10}},
									map[string]any{"type": "text", "props": map[string]any{"content": "{{msg}}"}},
								},
							},
						},
					},
					{"id": "footer", "type": "text", "props": map[string]any{"content": "Ready"}},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			t.Fatalf("layout error: %v", result.Content)
		}
		newM, _ := m.Update(DOMChangedMsg{Ctx: context.Background()})
		m = newM.(Model)

		// Populate with set_items.
		result, err = srv.CallTool(context.Background(), "set_items", map[string]any{
			"target": "log-list",
			"items": []map[string]any{
				{"level": "INFO", "msg": "server started"},
				{"level": "ERROR", "msg": "disk full"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			t.Fatalf("set_items error: %v", result.Content)
		}
		newM, _ = m.Update(DOMChangedMsg{Ctx: context.Background()})
		m = newM.(Model)

		view := m.View()
		for _, expected := range []string{"Log Viewer", "[INFO]", "server started", "[ERROR]", "disk full", "Ready"} {
			if !strings.Contains(view, expected) {
				t.Errorf("expected %q in view, got:\n%s", expected, view)
			}
		}
	})

	t.Run("log_viewer_demo_tree", func(t *testing.T) {
		// Replicate the exact tree Claude Code sends in the demo.
		m, srv := setupPipeline(t)
		m = replaceTree(t, m, srv, map[string]any{
			"id":    "root",
			"type":  "container",
			"props": map[string]any{"direction": "vertical"},
			"children": []map[string]any{
				{"id": "header", "type": "text", "props": map[string]any{"content": "Log Viewer", "style": "bold"}},
				{
					"id":    "main",
					"type":  "container",
					"props": map[string]any{"direction": "horizontal"},
					"children": []map[string]any{
						{
							"id":    "sidebar",
							"type":  "container",
							"props": map[string]any{"direction": "vertical", "width": "25%", "padding": 1},
							"children": []map[string]any{
								{"id": "filter-label", "type": "text", "props": map[string]any{"content": "Severity Filter", "style": "bold"}},
								{"id": "sev-error", "type": "button", "props": map[string]any{"label": "ERROR"}},
								{"id": "sev-warn", "type": "button", "props": map[string]any{"label": "WARN"}},
								{"id": "sev-info", "type": "button", "props": map[string]any{"label": "INFO"}},
								{"id": "sev-debug", "type": "button", "props": map[string]any{"label": "DEBUG"}},
								{"id": "search", "type": "input", "props": map[string]any{"placeholder": "Search logs..."}},
							},
						},
						{
							"id":    "log-view",
							"type":  "log",
							"props": map[string]any{"width": "fill", "sticky_bottom": true, "show_timestamps": true, "lines": []any{}},
						},
					},
				},
				{"id": "status-bar", "type": "text", "props": map[string]any{"content": "Loading...", "style": "dim"}},
			},
		})

		view := m.View()
		for _, expected := range []string{"Log Viewer", "Severity Filter", "ERROR", "WARN", "INFO", "DEBUG", "Search logs...", "Loading..."} {
			if !strings.Contains(view, expected) {
				t.Errorf("expected %q in log viewer demo view, got:\n%s", expected, view)
			}
		}
	})
}
