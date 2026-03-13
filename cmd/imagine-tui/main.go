// Package main is the entry point for the imagine-tui MCP server.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	"github.com/joncooper/imagine-tui/internal/render"
	"github.com/joncooper/imagine-tui/internal/widget"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		if err := serve(); err != nil {
			fmt.Fprintf(os.Stderr, "imagine-tui: %v\n", err)
			os.Exit(1)
		}
		return
	}
	fmt.Println("Usage: imagine-tui serve")
}

func serve() error {
	// Create the MCP server.
	srv, err := imcp.NewServer()
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	// Create the BubbleTea model.
	registry := widget.DefaultRegistry()
	model := render.NewModel(srv, registry)

	// Start the BubbleTea program.
	// Use stderr for TUI output since stdout is used by MCP stdio transport.
	program := tea.NewProgram(model,
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)

	// Create the bridge so MCP can notify BubbleTea of DOM changes.
	bridge := render.NewBridge(srv, program)
	bridge.OnMutation = func() {
		// Widget tree sync happens in the BubbleTea Update loop.
	}

	// Handle graceful shutdown on signals.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			srv.Shutdown()
			program.Send(render.ShutdownMsg{})
		case <-ctx.Done():
		}
	}()

	// Start the MCP server on stdio in a goroutine.
	mcpErrCh := make(chan error, 1)
	go func() {
		err := srv.MCPServer().Run(ctx, &mcp.StdioTransport{})
		mcpErrCh <- err

		// If MCP disconnects (broken pipe), notify BubbleTea.
		if err != nil && err != io.EOF {
			bridge.NotifyDisconnected(err)
		} else {
			bridge.NotifyDisconnected(nil)
		}
	}()

	// Run the BubbleTea program (blocks until quit).
	_, err = program.Run()
	cancel() // Stop MCP server when TUI exits.

	// Wait briefly for MCP server to finish.
	select {
	case mcpErr := <-mcpErrCh:
		if mcpErr != nil && mcpErr != io.EOF && mcpErr != context.Canceled {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", mcpErr)
		}
	default:
	}

	srv.Shutdown()
	return err
}
