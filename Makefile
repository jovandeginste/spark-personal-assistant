.PHONY: all vendor
all: test lint build
vendor:
	go mod tidy
	go mod vendor

test:
	CGO_ENABLED=0 go test -short -count 1 -tags "goolm,sqlite_foreign_keys" -mod vendor -covermode=atomic -gcflags=all=-l ./...

lint:
	CGO_ENABLED=0 golangci-lint run --allow-parallel-runners --fix --config=./.golangci.yml --build-tags="goolm,sqlite_foreign_keys"

build: build-spark build-mcp

build-spark:
	go build -o spark ./cmd/spark

build-mcp:
	go build -o spark-mcp-assistant ./cmd/spark-mcp-assistant

run-all:
	$(MAKE) run-mcp &
	$(MAKE) run-spark

run-spark:
	go run ./cmd/spark/ matrix

run-mcp:
	go run ./cmd/spark-mcp-assistant/

build-docker:
	docker build -t spark-personal-assistant --pull .
