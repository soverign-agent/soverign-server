.PHONY: all build test lint proto proto-check clean

# Proto generation settings
PROTO_DIR := shared/proto
PROTO_FILES := $(shell find $(PROTO_DIR) -name '*.proto' -not -path '$(PROTO_DIR)/google/*')
GO_OUT := .
PROTOC_OPTS := \
	-I=. -I=$(PROTO_DIR) \
	--go_out=$(GO_OUT) --go_opt=paths=source_relative \
	--go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
	--grpc-gateway_out=$(GO_OUT) --grpc-gateway_opt=paths=source_relative --grpc-gateway_opt=allow_delete_body=true

all: proto build test

build:
	go build ./...

test:
	go test -race ./...

lint:
	golangci-lint run ./...

proto:
	@echo "Generating Go code from protobuf definitions..."
	protoc $(PROTOC_OPTS) $(PROTO_FILES)

proto-check:
	@echo "Verifying protobuf compilation..."
	@protoc $(PROTOC_OPTS) $(PROTO_FILES) >/dev/null 2>&1 && echo "Proto compilation OK" || (echo "Proto compilation FAILED"; exit 1)

clean:
	find $(PROTO_DIR) -name '*.pb.go' -delete
