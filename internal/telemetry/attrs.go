package telemetry

import "go.opentelemetry.io/otel/attribute"

// Shared attribute keys used across imagine-tui runtime spans and logs.
const (
	AttrCommand        = "imagine.command"
	AttrComponent      = "imagine.component"
	AttrSessionID      = "imagine.session.id"
	AttrTransport      = "imagine.transport"
	AttrSocketPath     = "imagine.socket.path"
	AttrWidgetEvent    = "imagine.widget.event"
	AttrWidgetNodeID   = "imagine.widget.node_id"
	AttrHookName       = "imagine.hook.name"
	AttrMutationSource = "imagine.mutation.source"
)

// SessionID returns the common runtime session attribute.
func SessionID(sessionID uint64) attribute.KeyValue {
	return attribute.Int64(AttrSessionID, int64(sessionID))
}
