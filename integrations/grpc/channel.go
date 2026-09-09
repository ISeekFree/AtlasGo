package clawgrpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ClientOptions struct {
	Channels               map[string]ChannelOptions      `yaml:"channels"`
	DisableAuthPropagation bool                           `yaml:"disable-auth-propagation"`
	TokenHeaders           []string                       `yaml:"token-headers"`
	AdminTokenHeaders      []string                       `yaml:"admin-token-headers"`
	UnaryInterceptors      []grpc.UnaryClientInterceptor  `yaml:"-"`
	StreamInterceptors     []grpc.StreamClientInterceptor `yaml:"-"`
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
		return nil, fmt.Errorf("missing claw.grpc.client.channels.%s.target", name)
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
	if !f.options.DisableAuthPropagation {
		authOptions := ClientAuthOptions{
			TokenHeaders:      f.options.TokenHeaders,
			AdminTokenHeaders: f.options.AdminTokenHeaders,
		}
		unaryInterceptors = append([]grpc.UnaryClientInterceptor{UnaryClientAuthInterceptor(authOptions)}, unaryInterceptors...)
		streamInterceptors = append([]grpc.StreamClientInterceptor{StreamClientAuthInterceptor(authOptions)}, streamInterceptors...)
	}
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
