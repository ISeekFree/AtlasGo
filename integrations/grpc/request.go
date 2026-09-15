package atlasgrpc

import (
	"strings"

	"github.com/ISeekFree/AtlasGo/web"
	"google.golang.org/grpc/metadata"
)

// MetadataRequest adapts gRPC metadata onto the transport-neutral web.Request,
// so the application's ContextLoader parses a gRPC token exactly like an HTTP
// one. gRPC metadata keys are lower-case; lookups lower-case the requested
// name to match.
type MetadataRequest struct {
	MD metadata.MD
	IP string
}

func (r MetadataRequest) Header(name string) string {
	for _, value := range r.MD.Get(strings.ToLower(name)) {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (r MetadataRequest) Parameter(string) string { return "" }

func (r MetadataRequest) RemoteIP() string { return r.IP }

func (r MetadataRequest) Method() string { return "" }

func (r MetadataRequest) Path() string { return "" }

var _ web.Request = MetadataRequest{}
