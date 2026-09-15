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
	uid := webContextUID(ctx)
	return &demov1.EchoResponse{
		Message: "echo:" + uid + ":" + request.GetMessage(),
		UserId:  uid,
	}, nil
}

func (demoServiceServer) CurrentUser(ctx context.Context, _ *demov1.CurrentUserRequest) (*demov1.CurrentUserResponse, error) {
	wc, _ := atlasgrpc.ContextFromContext(ctx)
	if wc == nil {
		return &demov1.CurrentUserResponse{}, nil
	}
	return &demov1.CurrentUserResponse{
		UserId:      wc.UID,
		Domain:      wc.Domain,
		Permissions: demoGrantedPermissions(wc),
	}, nil
}

func webContextUID(ctx context.Context) string {
	wc, ok := atlasgrpc.ContextFromContext(ctx)
	if !ok || wc == nil {
		return ""
	}
	return wc.UID
}
