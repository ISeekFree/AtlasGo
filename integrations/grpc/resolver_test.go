package atlasgrpc

import (
	"net/url"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func TestStaticResolverSupportsDomainAndIPv6(t *testing.T) {
	for _, endpoint := range []string{"account.internal:16814", "[::1]:16814"} {
		parsed, err := url.Parse("static://" + endpoint)
		if err != nil {
			t.Fatalf("url.Parse(%q) error = %v", endpoint, err)
		}
		conn := &recordingClientConn{}
		_, err = staticResolverBuilder{}.Build(resolver.Target{URL: *parsed}, conn, resolver.BuildOptions{})
		if err != nil {
			t.Fatalf("Build(%q) error = %v", endpoint, err)
		}
		if len(conn.state.Addresses) != 1 || conn.state.Addresses[0].Addr != endpoint {
			t.Fatalf("Build(%q) addresses = %#v", endpoint, conn.state.Addresses)
		}
	}
}

func TestStaticResolverIsDefault(t *testing.T) {
	if got := resolver.GetDefaultScheme(); got != staticResolverScheme {
		t.Fatalf("default scheme = %q, want %q", got, staticResolverScheme)
	}
	for _, scheme := range []string{"dns", "unix", "passthrough"} {
		if resolver.Get(scheme) == nil {
			t.Fatalf("%s resolver is not registered", scheme)
		}
	}
	conn, err := grpc.NewClient("account.internal:16814",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	defer conn.Close()
	if got := conn.CanonicalTarget(); got != "static:///account.internal:16814" {
		t.Fatalf("canonical target = %q", got)
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
