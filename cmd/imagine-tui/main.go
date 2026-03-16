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
	"github.com/joncooper/imagine-tui/internal/render"
	"github.com/joncooper/imagine-tui/internal/telemetry"
	"github.com/joncooper/imagine-tui/internal/widget"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const version = "0.1.0"

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

func shutdownRuntime(rt *telemetry.Runtime) {
	if rt == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = rt.Shutdown(ctx)
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	socketPath := fs.String("socket", "", "Unix socket path for MCP transport (if omitted, uses stdio)")
	logPath := fs.String("log", "", "Log file path (if omitted, no logging)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rt, err := telemetry.New(context.Background(), telemetry.Config{
		ServiceName:    "imagine-tui",
		ServiceVersion: version,
		Command:        "serve",
		LogPath:        *logPath,
	})
	if err != nil {
		return err
	}
	defer shutdownRuntime(rt)

	logger := rt.Logger.With(
		slog.String(telemetry.AttrComponent, "cmd"),
	)
	cmdTracer := rt.Tracer("github.com/joncooper/imagine-tui/cmd/imagine-tui")
	renderTracer := rt.Tracer("github.com/joncooper/imagine-tui/internal/render")
	ctx, span := cmdTracer.Start(context.Background(), "command.serve")
	defer span.End()
	span.SetAttributes(attribute.String(telemetry.AttrSocketPath, *socketPath))

	logger.InfoContext(ctx, "starting imagine-tui", "socket", *socketPath)

	// Create the MCP server.
	srv, err := imcp.NewServer()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create server: %w", err)
	}
	srv.SetLogger(logger.With(slog.String(telemetry.AttrComponent, "mcp")))
	srv.SetTracer(rt.Tracer("github.com/joncooper/imagine-tui/internal/mcp"))

	if *socketPath != "" {
		return serveSocket(ctx, cmdTracer, renderTracer, srv, *socketPath, logger)
	}
	return serveStdio(ctx, cmdTracer, renderTracer, srv, logger)
}

// serveStdio runs MCP over stdin/stdout with BubbleTea on stderr.
func serveStdio(rootCtx context.Context, tracer, renderTracer trace.Tracer, srv *imcp.Server, logger *slog.Logger) error {
	ctx, span := tracer.Start(
		rootCtx,
		"serve.stdio",
		trace.WithAttributes(attribute.String(telemetry.AttrTransport, "stdio")),
	)
	defer span.End()

	registry := widget.DefaultRegistry()
	model := render.NewModel(srv, registry)
	model.SetLogger(logger.With(slog.String(telemetry.AttrComponent, "render")))
	model.SetTracer(renderTracer)

	// Use stderr for TUI output since stdout is used by MCP stdio transport.
	program := tea.NewProgram(model,
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)

	// Wire bridge: MCP mutations → BubbleTea re-renders.
	bridge := render.NewBridge(srv, program)
	srv.SetOnMutation(bridge.NotifyDOMChanged)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			logger.InfoContext(ctx, "signal received, shutting down")
			srv.Shutdown()
			program.Send(render.ShutdownMsg{})
		case <-ctx.Done():
		}
	}()

	// Start MCP on stdio.
	mcpErrCh := make(chan error, 1)
	go func() {
		sessionCtx, sessionSpan := tracer.Start(
			ctx,
			"mcp.session.stdio",
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				telemetry.SessionID(1),
				attribute.String(telemetry.AttrTransport, "stdio"),
			),
		)
		logger.InfoContext(sessionCtx, "MCP server starting on stdio")
		bridge.NotifyConnected(sessionCtx, 1)
		err := srv.MCPServer().Run(sessionCtx, &mcp.StdioTransport{})
		mcpErrCh <- err
		if err != nil && err != io.EOF && err != context.Canceled {
			sessionSpan.RecordError(err)
			sessionSpan.SetStatus(codes.Error, err.Error())
		}
		logger.InfoContext(sessionCtx, "MCP stdio session ended", "error", err)
		if err != nil && err != io.EOF {
			bridge.NotifyDisconnected(sessionCtx, 1, err)
		} else {
			bridge.NotifyDisconnected(sessionCtx, 1, nil)
		}
		sessionSpan.End()
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
func serveSocket(rootCtx context.Context, tracer, renderTracer trace.Tracer, srv *imcp.Server, socketPath string, logger *slog.Logger) error {
	ctx, span := tracer.Start(
		rootCtx,
		"serve.socket",
		trace.WithAttributes(
			attribute.String(telemetry.AttrTransport, "unix"),
			attribute.String(telemetry.AttrSocketPath, socketPath),
		),
	)
	defer span.End()

	// Clean up stale socket file.
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("remove stale socket: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("listen on %s: %w", socketPath, err)
	}
	defer func() { _ = listener.Close() }()
	defer func() { _ = os.Remove(socketPath) }()

	registry := widget.DefaultRegistry()
	model := render.NewModel(srv, registry)
	model.SetLogger(logger.With(slog.String(telemetry.AttrComponent, "render")))
	model.SetTracer(renderTracer)

	// BubbleTea uses stdout directly — no MCP contention on stdio.
	program := tea.NewProgram(model,
		tea.WithAltScreen(),
	)

	// Wire bridge: MCP mutations → BubbleTea re-renders.
	bridge := render.NewBridge(srv, program)
	srv.SetOnMutation(bridge.NotifyDOMChanged)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			logger.InfoContext(ctx, "signal received, shutting down")
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
				logger.DebugContext(ctx, "accept ended", "error", err)
				return // listener closed
			}
			logger.InfoContext(ctx, "client connected", "remote", conn.RemoteAddr())
			go handleConnection(ctx, tracer, srv, bridge, owners, conn, logger)
		}
	}()

	logger.InfoContext(ctx, "listening", "socket", socketPath)
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

func handleConnection(ctx context.Context, tracer trace.Tracer, srv *imcp.Server, bridge *render.Bridge, owners *ownerGate, conn net.Conn, logger *slog.Logger) {
	defer func() { _ = conn.Close() }()

	sessionID, ok := owners.TryClaim()
	if !ok {
		logger.WarnContext(ctx, "owner rejected", "remote", conn.RemoteAddr())
		return
	}
	sessionCtx, sessionSpan := tracer.Start(
		ctx,
		"mcp.session.socket",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			telemetry.SessionID(sessionID),
			attribute.String(telemetry.AttrTransport, "unix"),
			attribute.String("net.peer.address", conn.RemoteAddr().String()),
		),
	)
	defer sessionSpan.End()

	transport := &mcp.IOTransport{
		Reader: conn,
		Writer: conn,
	}

	session, err := srv.MCPServer().Connect(sessionCtx, transport, nil)
	if err != nil {
		owners.Release(sessionID)
		sessionSpan.RecordError(err)
		sessionSpan.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(sessionCtx, "MCP connect failed", "error", err)
		return
	}

	bridge.NotifyConnected(sessionCtx, sessionID)
	if sessionID == 1 {
		logger.InfoContext(sessionCtx, "owner attached", "session_id", sessionID)
	} else {
		logger.InfoContext(sessionCtx, "owner reattached", "session_id", sessionID)
	}

	err = session.Wait()
	if err != nil && err != io.EOF && err != context.Canceled {
		sessionSpan.RecordError(err)
		sessionSpan.SetStatus(codes.Error, err.Error())
	}
	logger.InfoContext(sessionCtx, "MCP session ended", "session_id", sessionID, "error", err)
	if owners.Release(sessionID) {
		bridge.NotifyDisconnected(sessionCtx, sessionID, err)
	}
}

// connectCmd dials a Unix socket and bridges stdin/stdout to it.
// Used by Claude Code's .mcp.json: {"command": "imagine-tui", "args": ["connect", "<socket>"]}
func connectCmd(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: imagine-tui connect <socket-path>")
	}
	socketPath := args[0]

	rt, err := telemetry.New(context.Background(), telemetry.Config{
		ServiceName:    "imagine-tui",
		ServiceVersion: version,
		Command:        "connect",
	})
	if err != nil {
		return err
	}
	defer shutdownRuntime(rt)

	logger := rt.Logger.With(slog.String(telemetry.AttrComponent, "cmd"))
	tracer := rt.Tracer("github.com/joncooper/imagine-tui/cmd/imagine-tui")
	ctx, span := tracer.Start(
		context.Background(),
		"command.connect",
		trace.WithAttributes(attribute.String(telemetry.AttrSocketPath, socketPath)),
	)
	defer span.End()

	// Check if the socket file exists before attempting to dial.
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("socket %s does not exist — start the server first: imagine-tui serve -socket %s", socketPath, socketPath)
	}

	dialCtx, dialSpan := tracer.Start(
		ctx,
		"connect.dial",
		trace.WithAttributes(
			attribute.String(telemetry.AttrTransport, "unix"),
			attribute.String(telemetry.AttrSocketPath, socketPath),
		),
	)
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		dialSpan.RecordError(err)
		dialSpan.SetStatus(codes.Error, err.Error())
	} else {
		logger.InfoContext(dialCtx, "connected to socket", "socket", socketPath)
	}
	dialSpan.End()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("dial %s: %w (is the server running?)", socketPath, err)
	}
	defer func() { _ = conn.Close() }()

	// Bidirectional copy: stdin → socket, socket → stdout.
	errCh := make(chan error, 2)
	go func() {
		_, err := copyWithSpan(ctx, tracer, "connect.copy.stdin_to_socket", conn, os.Stdin)
		errCh <- err
	}()
	go func() {
		_, err := copyWithSpan(ctx, tracer, "connect.copy.socket_to_stdout", os.Stdout, conn)
		errCh <- err
	}()

	// Wait for either direction to finish.
	err = <-errCh
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func copyWithSpan(ctx context.Context, tracer trace.Tracer, name string, dst io.Writer, src io.Reader) (int64, error) {
	_, span := tracer.Start(ctx, name)
	defer span.End()

	n, err := io.Copy(dst, src)
	span.SetAttributes(attribute.Int64("io.copy.bytes", n))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return n, err
}
