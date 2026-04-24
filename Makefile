.PHONY: all build test lint proto proto-check clean start-dev

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

start-dev:
	./scripts/start-dev.sh

infra:
	./scripts/start-dev.sh --infra

stop-dev:
	./scripts/start-dev.sh --stop

logs-dev:
	./scripts/start-dev.sh --logs

status-dev:
	./scripts/start-dev.sh --status
