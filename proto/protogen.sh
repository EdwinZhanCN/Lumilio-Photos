protoc --go_out=. --go-grpc_out=. ml_service.proto
python -m grpc_tools.protoc -I . --python_out=. --pyi_out=. --grpc_python_out=. ml_service.proto
