package atlasgrpc

import (
	"fmt"
	"net"
	"strings"

	"google.golang.org/grpc/resolver"
)

const staticResolverScheme = "static"

func init() {
	resolver.Register(staticResolverBuilder{})
	resolver.SetDefaultScheme(staticResolverScheme)
}

type staticResolverBuilder struct{}

func (staticResolverBuilder) Scheme() string {
	return staticResolverScheme
}

func (staticResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	endpoint := target.Endpoint()
	if endpoint == "" {
		endpoint = target.URL.Host
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("static target must be static://host:port")
	}
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		return nil, fmt.Errorf("static target must be static://host:port: %w", err)
	}
	staticResolver := &staticResolver{target: endpoint, cc: cc}
	staticResolver.ResolveNow(resolver.ResolveNowOptions{})
	return staticResolver, nil
}

func (staticResolverBuilder) OverrideAuthority(target resolver.Target) string {
	endpoint := target.Endpoint()
	if endpoint == "" {
		return target.URL.Host
	}
	return endpoint
}

type staticResolver struct {
	target string
	cc     resolver.ClientConn
}

func (r *staticResolver) ResolveNow(resolver.ResolveNowOptions) {
	_ = r.cc.UpdateState(resolver.State{
		Addresses: []resolver.Address{{Addr: r.target}},
	})
}

func (r *staticResolver) Close() {
}
