package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func pushItemsCmd(args []string) error {
	fs := flag.NewFlagSet("push-items", flag.ContinueOnError)
	socketPath := fs.String("socket", "", "Unix socket path exposed by imagine-tui serve -socket")
	target := fs.String("target", "", "Target node ID for set_items")
	filePath := fs.String("file", "", "Path to a JSON array or JSONL/NDJSON file")
	format := fs.String("format", "", "Optional file format override: auto, json, jsonl, or ndjson")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: imagine-tui push-items --socket <socket-path> --target <id> --file <path> [--format json|jsonl|ndjson]")
	}
	if strings.TrimSpace(*socketPath) == "" {
		return fmt.Errorf("missing required flag: --socket")
	}
	if strings.TrimSpace(*target) == "" {
		return fmt.Errorf("missing required flag: --target")
	}
	if strings.TrimSpace(*filePath) == "" {
		return fmt.Errorf("missing required flag: --file")
	}

	items, err := loadItemsFromFile(*filePath, *format)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialSocket(*socketPath)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	clientTransport := &mcp.IOTransport{Reader: conn, Writer: conn}
	client := mcp.NewClient(&mcp.Implementation{Name: "imagine-tui-push-items", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", *socketPath, err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "set_items",
		Arguments: map[string]any{
			"target": *target,
			"items":  items,
		},
	})
	if err != nil {
		return fmt.Errorf("set_items: %w", err)
	}

	text, err := callToolText(result)
	if err != nil {
		return err
	}
	if result.IsError {
		return fmt.Errorf("set_items: %s", text)
	}

	if _, err := fmt.Fprintln(os.Stdout, text); err != nil {
		return fmt.Errorf("write push-items result: %w", err)
	}
	return nil
}

func dialSocket(socketPath string) (net.Conn, error) {
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("socket %s does not exist — start the server first: imagine-tui serve -socket %s", socketPath, socketPath)
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w (is the server running?)", socketPath, err)
	}
	return conn, nil
}

func callToolText(result *mcp.CallToolResult) (string, error) {
	if len(result.Content) == 0 {
		return "", fmt.Errorf("tool result has no content")
	}
	data, err := json.Marshal(result.Content[0])
	if err != nil {
		return "", fmt.Errorf("marshal tool result content: %w", err)
	}
	var wire struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return "", fmt.Errorf("unmarshal tool result content: %w", err)
	}
	return wire.Text, nil
}

func loadItemsFromFile(path, format string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("push-items: read file %q: %w", path, err)
	}

	format = normalizeItemFormat(format)
	switch format {
	case "", "auto":
		return loadItemsAuto(path, data)
	case "json":
		return loadItemsJSON(path, data)
	case "jsonl", "ndjson":
		return loadItemsJSONL(path, data)
	default:
		return nil, fmt.Errorf("push-items: unsupported file format %q for %q", format, path)
	}
}

func normalizeItemFormat(format string) string {
	return strings.ToLower(strings.TrimSpace(format))
}

func loadItemsAuto(path string, data []byte) ([]map[string]any, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return loadItemsJSON(path, data)
	case ".jsonl", ".ndjson":
		return loadItemsJSONL(path, data)
	}

	if items, err := loadItemsJSON(path, data); err == nil {
		return items, nil
	}
	if items, err := loadItemsJSONL(path, data); err == nil {
		return items, nil
	}

	if looksLikeRawLog(data) {
		return nil, fmt.Errorf("push-items: raw log parsing is not supported yet for %q; convert the file to JSON or JSONL first", path)
	}
	return nil, fmt.Errorf("push-items: could not detect a supported format for %q; supported formats are json, jsonl, and ndjson", path)
}

func loadItemsJSON(path string, data []byte) ([]map[string]any, error) {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("push-items: parse json file %q: %w", path, err)
	}
	if items == nil {
		return []map[string]any{}, nil
	}
	return items, nil
}

func loadItemsJSONL(path string, data []byte) ([]map[string]any, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	items := make([]map[string]any, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("push-items: parse jsonl file %q line %d: %w", path, lineNo, err)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("push-items: scan jsonl file %q: %w", path, err)
	}
	return items, nil
}

func looksLikeRawLog(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return false
	}
	return !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{")
}
