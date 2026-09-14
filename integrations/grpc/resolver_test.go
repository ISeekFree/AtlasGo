package atlasgrpc

import (
	"net/url"
	"testing"

	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

func TestStaticResolverBuildsSingleAddress(t *testing.T) {
	parsed, err := url.Parse("static://127.0.0.1:16814")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	conn := &recordingClientConn{}

	_, err = staticResolverBuilder{}.Build(resolver.Target{URL: *parsed}, conn, resolver.BuildOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(conn.state.Addresses) != 1 || conn.state.Addresses[0].Addr != "127.0.0.1:16814" {
		t.Fatalf("addresses = %#v", conn.state.Addresses)
	}
}

func TestStaticResolverRejectsMissingPort(t *testing.T) {
	parsed, err := url.Parse("static://127.0.0.1")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	_, err = staticResolverBuilder{}.Build(resolver.Target{URL: *parsed}, &recordingClientConn{}, resolver.BuildOptions{})
	if err == nil {
		t.Fatal("Build() error = nil, want error")
	}
}

func TestParseConfigSupportsDefaultOnlyPlaceholderTarget(t *testing.T) {
	config, err := ParseConfig([]byte(`
framework:
  grpc:
    client:
      channels:
        local:
          target: ${:static://127.0.0.1:16814}
`))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if got := config.Client.Channels["local"].Target; got != "static://127.0.0.1:16814" {
		t.Fatalf("target = %q", got)
	}
}

type recordingClientConn struct {
	state resolver.State
}

func (c *recordingClientConn) UpdateState(state resolver.State) error {
	c.state = state
	return nil
}

func (c *recordingClientConn) ReportError(error) {
}

func (c *recordingClientConn) NewAddress([]resolver.Address) {
}

func (c *recordingClientConn) NewServiceConfig(string) {
}

func (c *recordingClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return nil
}
