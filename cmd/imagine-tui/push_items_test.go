package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLoadItemsFromFileJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.json")
	data := `[
		{"id":"item-1","label":"First item"},
		{"id":"item-2","label":"Second item"}
	]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := loadItemsFromFile(path, "")
	if err != nil {
		t.Fatalf("loadItemsFromFile: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0]["id"] != "item-1" {
		t.Fatalf("items[0].id = %v, want item-1", items[0]["id"])
	}
}

func TestLoadItemsFromFileJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.jsonl")
	data := `{"id":"item-1","label":"First item"}
{"id":"item-2","label":"Second item"}
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := loadItemsFromFile(path, "")
	if err != nil {
		t.Fatalf("loadItemsFromFile: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[1]["label"] != "Second item" {
		t.Fatalf("items[1].label = %v, want Second item", items[1]["label"])
	}
}

func TestLoadItemsFromFileRawLogRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	data := `2026-03-16T12:00:00Z INFO [api] started
2026-03-16T12:00:01Z ERROR [api] failed`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadItemsFromFile(path, "")
	if err == nil {
		t.Fatal("expected raw log parsing to be rejected")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected non-empty error")
	}
}

func TestPushItemsCommand(t *testing.T) {
	socketPath, _, cleanup := startServePTY(t)
	defer cleanup()

	session, closeSession := connectSocketSession(t, socketPath)
	layoutList(t, session, "main-list")
	closeSession()

	path := filepath.Join(t.TempDir(), "items.jsonl")
	data := `{"id":"item-1","label":"First item"}
{"id":"item-2","label":"Second item"}
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(testBinaryPath(t),
		"push-items",
		"--socket", socketPath,
		"--target", "main-list",
		"--file", path,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("push-items failed: %v\n%s", err, output)
	}

	var result struct {
		OK    bool    `json:"ok"`
		Count float64 `json:"count"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("parse push-items output: %v\nraw: %s", err, output)
	}
	if !result.OK || result.Count != 2 {
		t.Fatalf("push-items output = %+v, want ok true count 2", result)
	}

	session, closeSession = connectSocketSession(t, socketPath)
	defer closeSession()

	queryResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "query",
		Arguments: map[string]any{"ids": []any{"main-list"}},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if queryResult.IsError {
		t.Fatalf("query error: %s", toolResultText(t, queryResult))
	}

	var query struct {
		Results map[string]struct {
			Props map[string]any `json:"props"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, queryResult)), &query); err != nil {
		t.Fatalf("parse query result: %v", err)
	}

	items, ok := query.Results["main-list"].Props["items"].([]any)
	if !ok {
		t.Fatalf("main-list items type = %T, want []any", query.Results["main-list"].Props["items"])
	}
	if len(items) != 2 {
		t.Fatalf("len(main-list.items) = %d, want 2", len(items))
	}
}

func connectSocketSession(t *testing.T, socketPath string) (session *mcp.ClientSession, closeFn func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		cancel()
		t.Fatalf("dial socket: %v", err)
	}

	clientTransport := &mcp.IOTransport{Reader: conn, Writer: conn}
	client := mcp.NewClient(&mcp.Implementation{Name: "push-items-test", Version: "1.0"}, nil)
	session, err = client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		_ = conn.Close()
		t.Fatalf("client connect: %v", err)
	}

	closeFn = func() {
		_ = session.Close()
		_ = conn.Close()
		cancel()
		time.Sleep(50 * time.Millisecond)
	}
	return session, closeFn
}

func layoutList(t *testing.T, session *mcp.ClientSession, listID string) {
	t.Helper()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "layout",
		Arguments: map[string]any{
			"tree": map[string]any{
				"id":   "root",
				"type": "container",
				"children": []any{
					map[string]any{
						"id":   listID,
						"type": "list",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	if result.IsError {
		t.Fatalf("layout error: %s", toolResultText(t, result))
	}
}
