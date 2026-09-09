#!/usr/bin/env bash
set -euo pipefail

# Regenerate Go protobuf messages and gRPC stubs for the demo module.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="${ROOT_DIR}/demo"

if command -v go >/dev/null 2>&1; then
	GOPATH_BIN="$(go env GOPATH)/bin"
	case ":${PATH}:" in
		*":${GOPATH_BIN}:"*) ;;
		*) export PATH="${GOPATH_BIN}:${PATH}" ;;
	esac
fi

missing=0

if ! command -v protoc >/dev/null 2>&1; then
	echo "missing protoc; install with: brew install protobuf" >&2
	missing=1
fi

if ! command -v protoc-gen-go >/dev/null 2>&1; then
	echo "missing protoc-gen-go; install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10" >&2
	missing=1
fi

if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
	echo "missing protoc-gen-go-grpc; install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1" >&2
	missing=1
fi

if [[ "${missing}" -ne 0 ]]; then
	exit 1
fi

cd "${DEMO_DIR}"

proto_files=()
while IFS= read -r proto_file; do
	proto_files+=("${proto_file}")
done < <(find proto -name '*.proto' -print | sort)

if [[ "${#proto_files[@]}" -eq 0 ]]; then
	echo "no proto files found under ${DEMO_DIR}/proto" >&2
	exit 1
fi

protoc \
	--proto_path=proto \
	--go_out=. \
	--go_opt=module=iseekfree.com/common/sdk/gomvc/demo \
	--go-grpc_out=. \
	--go-grpc_opt=module=iseekfree.com/common/sdk/gomvc/demo \
	"${proto_files[@]}"

echo "generated Go protobuf code for ${#proto_files[@]} proto file(s)"
