package clawgrpc

import (
	"testing"
	"time"
)

func TestParseConfigReadsServerAndNamedChannels(t *testing.T) {
	t.Setenv("GRPC_TEST_PORT", "19091")

	config, err := ParseConfig([]byte(`
claw:
  grpc:
    server:
      host: 127.0.0.1
      port: ${GRPC_TEST_PORT:19090}
      auth:
        required: true
        inner-token: internal
    client:
      channels:
        local:
          target: 127.0.0.1:19091
          plaintext: true
          dial-timeout: 3s
`))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if config.Server.ListenAddress() != "127.0.0.1:19091" || !config.Server.Auth.Required {
		t.Fatalf("server config = %#v", config.Server)
	}
	local := config.Client.Channels["local"]
	if local.Target != "127.0.0.1:19091" || !local.Plaintext || local.DialTimeout != 3*time.Second {
		t.Fatalf("local channel = %#v", local)
	}
}
