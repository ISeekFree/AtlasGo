package atlasgrpc

import (
	"testing"

	"google.golang.org/grpc/resolver"
)

func TestXDSResolverIsRegistered(t *testing.T) {
	if resolver.Get("xds") == nil {
		t.Fatal("xds resolver is not registered")
	}
}
