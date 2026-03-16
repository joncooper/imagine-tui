// Package main is the entry point for the imagine-tui MCP server.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	imcp "github.com/joncooper/imagine-tui/internal/mcp"
	iotel "github.com/joncooper/imagine-tui/internal/otel"
	"github.com/joncooper/imagine-tui/internal/render"
	"github.com/joncooper/imagine-tui/internal/widget"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: imagine-tui <serve|connect> [flags]")
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "connect":
		err = connectCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: imagine-tui <serve|connect> [flags]\n", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "imagine-tui %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
}

// initLogger sets up slog to write JSON to the given file path.
// If logPath is empty, logging is discarded.
func initLogger(logPath string) (*slog.Logger, func(), error) {
	if logPath == "" {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}, nil
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, func() { _ = f.Close() }, nil
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	socketPath := fs.String("socket", "", "Unix socket path for MCP transport (if omitted, uses stdio)")
	logPath := fs.String("log", "", "Log file path (if omitted, no logging)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logger, closeLog, err := initLogger(*logPath)
	if err != nil {
		return err
	}
	defer closeLog()

	logger.Info("starting imagine-tui", "socket", *socketPath)

	// Initialize OpenTelemetry (no-op when OTEL_EXPORTER_OTLP_ENDPOINT is unset).
	otelShutdown, err := iotel.Init(context.Background(), "imagine-tui", "0.1.0")
	if err != nil {
		return fmt.Errorf("init otel: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelShutdown(ctx)
	}()

	// Create the MCP server.
	srv, err := imcp.NewServer()
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	srv.SetLogger(logger)

	if *socketPath != "" {
		return serveSocket(srv, *socketPath, logger)
	}
	return serveStdio(srv, logger)
}

// serveStdio runs MCP over stdin/stdout with BubbleTea on stderr.
func serveStdio(srv *imcp.Server, logger *slog.Logger) error {
	registry := widget.DefaultRegistry()
	model := render.NewModel(srv, registry)
	model.SetLogger(logger)

	// Use stderr for TUI output since stdout is used by MCP stdio transport.
	program := tea.NewProgram(model,
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)

	// Wire bridge: MCP mutations → BubbleTea re-renders.
	bridge := render.NewBridge(srv, program)
	srv.SetOnMutation(bridge.NotifyDOMChanged)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			logger.Info("signal received, shutting down")
			srv.Shutdown()
			program.Send(render.ShutdownMsg{})
		case <-ctx.Done():
		}
	}()

	// Start MCP on stdio.
	mcpErrCh := make(chan error, 1)
	go func() {
		logger.Info("MCP server starting on stdio")
		bridge.NotifyConnected(1)
		err := srv.MCPServer().Run(ctx, &mcp.StdioTransport{})
		mcpErrCh <- err
		logger.Info("MCP stdio session ended", "error", err)
		if err != nil && err != io.EOF {
			bridge.NotifyDisconnected(1, err)
		} else {
			bridge.NotifyDisconnected(1, nil)
		}
	}()

	// Run BubbleTea (blocks until quit).
	_, err := program.Run()
	cancel()

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

// serveSocket runs MCP over a Unix domain socket with BubbleTea on stdout.
func serveSocket(srv *imcp.Server, socketPath string, logger *slog.Logger) error {
	// Clean up stale socket file.
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", socketPath, err)
	}
	defer func() { _ = listener.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	registry := widget.DefaultRegistry()
	model := render.NewModel(srv, registry)
	model.SetLogger(logger)

	// BubbleTea uses stdout directly — no MCP contention on stdio.
	program := tea.NewProgram(model,
		tea.WithAltScreen(),
	)

	// Wire bridge: MCP mutations → BubbleTea re-renders.
	bridge := render.NewBridge(srv, program)
	srv.SetOnMutation(bridge.NotifyDOMChanged)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			logger.Info("signal received, shutting down")
			srv.Shutdown()
			program.Send(render.ShutdownMsg{})
			_ = listener.Close()
		case <-ctx.Done():
		}
	}()

	// Accept MCP connections on the Unix socket.
	owners := &ownerGate{}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				logger.Debug("accept ended", "error", err)
				return // listener closed
			}
			logger.Info("client connected", "remote", conn.RemoteAddr())
			go handleConnection(ctx, srv, bridge, owners, conn, logger)
		}
	}()

	logger.Info("listening", "socket", socketPath)
	fmt.Fprintf(os.Stderr, "Listening on %s\n", socketPath)

	// Run BubbleTea (blocks until quit).
	_, err = program.Run()
	cancel()
	srv.Shutdown()
	return err
}

type ownerGate struct {
	mu     sync.Mutex
	next   uint64
	active uint64
}

func (g *ownerGate) TryClaim() (uint64, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active != 0 {
		return 0, false
	}
	g.next++
	g.active = g.next
	return g.active, true
}

func (g *ownerGate) Release(id uint64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if id == 0 || g.active != id {
		return false
	}
	g.active = 0
	return true
}

func handleConnection(ctx context.Context, srv *imcp.Server, bridge *render.Bridge, owners *ownerGate, conn net.Conn, logger *slog.Logger) {
	defer func() { _ = conn.Close() }()

	sessionID, ok := owners.TryClaim()
	if !ok {
		logger.Warn("owner rejected", "remote", conn.RemoteAddr())
		return
	}

	transport := &mcp.IOTransport{
		Reader: conn,
		Writer: conn,
	}

	session, err := srv.MCPServer().Connect(ctx, transport, nil)
	if err != nil {
		owners.Release(sessionID)
		logger.Error("MCP connect failed", "error", err)
		return
	}

	bridge.NotifyConnected(sessionID)
	if sessionID == 1 {
		logger.Info("owner attached", "session_id", sessionID)
	} else {
		logger.Info("owner reattached", "session_id", sessionID)
	}

	err = session.Wait()
	logger.Info("MCP session ended", "session_id", sessionID, "error", err)
	if owners.Release(sessionID) {
		bridge.NotifyDisconnected(sessionID, err)
	}
}

// connectCmd dials a Unix socket and bridges stdin/stdout to it.
// Used by Claude Code's .mcp.json: {"command": "imagine-tui", "args": ["connect", "<socket>"]}
func connectCmd(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: imagine-tui connect <socket-path>")
	}
	socketPath := args[0]

	// Check if the socket file exists before attempting to dial.
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return fmt.Errorf("socket %s does not exist — start the server first: imagine-tui serve -socket %s", socketPath, socketPath)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("dial %s: %w (is the server running?)", socketPath, err)
	}
	defer func() { _ = conn.Close() }()

	// Bidirectional copy: stdin → socket, socket → stdout.
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		errCh <- err
	}()

	// Wait for either direction to finish.
	return <-errCh
}
