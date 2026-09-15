package web

// WebsocketAttributeKey is the connection-state key holding the resolved
// *Context of a WebSocket session.
const WebsocketAttributeKey = "atlas.websocket.context"

// WebsocketContext is the WebSocket base context: the WebSocket counterpart of
// the HTTP Context and of the gRPC carrier in integrations/grpc.
//
// A session outlives a single handler call, so the context is resolved once
// (typically during the handshake through the same ContextLoader used by HTTP
// and gRPC) and kept on the connection state. Per frame, bind it back onto the
// call context with web.WithContext and read it with web.ContextFromContext.
//
// Applications may use web.Context directly, embed it in their own struct, or
// embed WebsocketContext when they need session-level fields such as SessionID.
type WebsocketContext struct {
	*Context
	SessionID string
}

// AttachWebsocketContext stores the resolved context on the connection state.
func AttachWebsocketContext(attributes map[string]any, wc *Context) {
	if attributes == nil || wc == nil {
		return
	}
	attributes[WebsocketAttributeKey] = wc
}

// WebsocketContextFrom reads the context previously attached to the connection
// state, if any.
func WebsocketContextFrom(attributes map[string]any) (*Context, bool) {
	if attributes == nil {
		return nil, false
	}
	wc, ok := attributes[WebsocketAttributeKey].(*Context)
	return wc, ok
}
