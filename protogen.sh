protoc -I proto \
  --plugin=protoc-gen-go="$(go env GOPATH)/bin/protoc-gen-go" \
  --plugin=protoc-gen-go-grpc="$(go env GOPATH)/bin/protoc-gen-go-grpc" \
  --go_out=paths=source_relative:server/proto \
  --go-grpc_out=paths=source_relative:server/proto \
  proto/ml_service.proto
