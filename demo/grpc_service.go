package main

import (
	"context"

	demov1 "iseekfree.com/common/sdk/gomvc/demo/gen/demo/v1"
	clawgrpc "iseekfree.com/common/sdk/gomvc/grpc"
)

type demoServiceServer struct {
	demov1.UnimplementedDemoServiceServer
}

func (demoServiceServer) Echo(ctx context.Context, request *demov1.EchoRequest) (*demov1.EchoResponse, error) {
	identity, _ := clawgrpc.IdentityFromContext(ctx)
	return &demov1.EchoResponse{
		Message: "echo:" + identity.UserID + ":" + request.GetMessage(),
		UserId:  identity.UserID,
	}, nil
}

func (demoServiceServer) CurrentUser(ctx context.Context, _ *demov1.CurrentUserRequest) (*demov1.CurrentUserResponse, error) {
	identity, _ := clawgrpc.IdentityFromContext(ctx)
	return &demov1.CurrentUserResponse{
		UserId:      identity.UserID,
		Domain:      identity.Domain,
		Permissions: append([]string(nil), identity.Permissions...),
	}, nil
}
