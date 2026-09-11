package main

import (
	"context"

	demov1 "github.com/ISeekFree/AtlasGo/demo/gen/demo/v1"
	atlasgrpc "github.com/ISeekFree/AtlasGo/integrations/grpc"
)

type demoServiceServer struct {
	demov1.UnimplementedDemoServiceServer
}

func (demoServiceServer) Echo(ctx context.Context, request *demov1.EchoRequest) (*demov1.EchoResponse, error) {
	identity, _ := atlasgrpc.IdentityFromContext(ctx)
	return &demov1.EchoResponse{
		Message: "echo:" + identity.UserID + ":" + request.GetMessage(),
		UserId:  identity.UserID,
	}, nil
}

func (demoServiceServer) CurrentUser(ctx context.Context, _ *demov1.CurrentUserRequest) (*demov1.CurrentUserResponse, error) {
	identity, _ := atlasgrpc.IdentityFromContext(ctx)
	return &demov1.CurrentUserResponse{
		UserId:      identity.UserID,
		Domain:      identity.Domain,
		Permissions: append([]string(nil), identity.Permissions...),
	}, nil
}
