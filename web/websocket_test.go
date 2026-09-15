package web

import "testing"

func TestWebsocketContextAttachAndRead(t *testing.T) {
	state := map[string]any{}
	wc := &Context{UID: "u1"}
	AttachWebsocketContext(state, wc)

	got, ok := WebsocketContextFrom(state)
	if !ok || got != wc || got.UID != "u1" {
		t.Fatalf("context = %#v, ok = %v", got, ok)
	}

	ws := &WebsocketContext{Context: wc, SessionID: "s-1"}
	if ws.UID != "u1" || ws.SessionID != "s-1" {
		t.Fatalf("embedded context = %#v", ws)
	}
}

func TestWebsocketContextFromMissingState(t *testing.T) {
	if _, ok := WebsocketContextFrom(nil); ok {
		t.Fatalf("a nil state must not resolve a context")
	}
	if _, ok := WebsocketContextFrom(map[string]any{}); ok {
		t.Fatalf("an empty state must not resolve a context")
	}

	AttachWebsocketContext(nil, &Context{UID: "u1"})
	AttachWebsocketContext(map[string]any{}, nil)
}
