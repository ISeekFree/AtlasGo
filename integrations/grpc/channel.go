package atlasgrpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientOptions configures the named client channels and the interceptors
// mounted on them.
//
// AtlasGo deliberately does not install an auth interceptor. Which HTTP header
// carries the caller token and which gRPC metadata key a downstream service
// expects are business contracts, so the consuming project supplies its own
// grpc.UnaryClientInterceptor / grpc.StreamClientInterceptor values and reads
// whatever token source it uses.
type ClientOptions struct {
	Channels           map[string]ChannelOptions      `yaml:"channels"`
	UnaryInterceptors  []grpc.UnaryClientInterceptor  `yaml:"-"`
	StreamInterceptors []grpc.StreamClientInterceptor `yaml:"-"`
}

type ChannelOptions struct {
	Target                string        `yaml:"target"`
	Plaintext             bool          `yaml:"plaintext"`
	DialTimeout           time.Duration `yaml:"dial-timeout"`
	MaxInboundMessageSize int           `yaml:"max-inbound-message-size"`
}

type ChannelFactory struct {
	options ClientOptions
	conns   map[string]*grpc.ClientConn
}

func NewChannelFactory(options ClientOptions) *ChannelFactory {
	return &ChannelFactory{options: options, conns: map[string]*grpc.ClientConn{}}
}

func (f *ChannelFactory) Channel(ctx context.Context, name string) (*grpc.ClientConn, error) {
	if conn, ok := f.conns[name]; ok {
		return conn, nil
	}
	config, ok := f.options.Channels[name]
	if !ok || config.Target == "" {
		return nil, fmt.Errorf("missing framework.grpc.client.channels.%s.target", name)
	}
	dialOptions := []grpc.DialOption{}
	if config.Plaintext {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if config.MaxInboundMessageSize > 0 {
		dialOptions = append(dialOptions, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(config.MaxInboundMessageSize)))
	}
	unaryInterceptors := append([]grpc.UnaryClientInterceptor{}, f.options.UnaryInterceptors...)
	streamInterceptors := append([]grpc.StreamClientInterceptor{}, f.options.StreamInterceptors...)
	if len(unaryInterceptors) > 0 {
		dialOptions = append(dialOptions, grpc.WithChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		dialOptions = append(dialOptions, grpc.WithChainStreamInterceptor(streamInterceptors...))
	}
	dialCtx := ctx
	cancel := func() {}
	if config.DialTimeout > 0 {
		dialCtx, cancel = context.WithTimeout(ctx, config.DialTimeout)
	}
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, config.Target, dialOptions...)
	if err != nil {
		return nil, err
	}
	f.conns[name] = conn
	return conn, nil
}

func (f *ChannelFactory) Close() error {
	var first error
	for name, conn := range f.conns {
		if err := conn.Close(); err != nil && first == nil {
			first = fmt.Errorf("close %s: %w", name, err)
		}
		delete(f.conns, name)
	}
	return first
}
