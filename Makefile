.PHONY: all vendor down-docker
all: test lint build
vendor:
	go mod tidy
	go mod vendor

update-deps:
	go get -u ./...
	go mod tidy
	go mod vendor

test:
	CGO_ENABLED=0 go test -short -count 1 -tags "goolm,sqlite_foreign_keys" -mod vendor -covermode=atomic -gcflags=all=-l ./...

lint:
	CGO_ENABLED=0 golangci-lint run --allow-parallel-runners --fix --config=./.golangci.yml --build-tags="goolm,sqlite_foreign_keys"

install-hooks:
	printf "#!/bin/sh\nset -e\n\necho 'Running tests...'\nmake test\n\necho 'Running linter...'\nmake lint\n" > .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit

build: build-spark build-mcp

build-spark:
	go build -o data/spark ./cmd/spark

build-mcp:
	go build -o data/spark-mcp-assistant ./cmd/spark-mcp-assistant
	go build -o data/spark-mcp-hockey ./cmd/spark-mcp-hockey

run-all:
	$(MAKE) run-assistant &
	$(MAKE) run-hockey &
	$(MAKE) run-spark

run-spark:
	go run ./cmd/spark/ router

run-hockey:
	go run ./cmd/spark-mcp-hockey/

run-assistant:
	go run ./cmd/spark-mcp-assistant/

build-docker:
	docker compose build

build-docker-spark:
	docker compose build mcp-assistant

build-docker-mcp:
	docker compose build mcp-assistant mcp-hockey

run-docker: build-docker
	docker compose up -d --remove-orphans

down-docker:
	docker compose down --remove-orphans
