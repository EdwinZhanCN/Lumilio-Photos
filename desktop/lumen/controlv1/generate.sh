#!/bin/sh
# Regenerates the Go stubs for the mirrored lumen.control.v1 proto.
set -e
cd "$(dirname "$0")"
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  control.proto
