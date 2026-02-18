.PHONY: all vendor
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

build: build-spark build-mcp

build-spark:
	go build -o data/spark ./cmd/spark

build-mcp:
	go build -o data/spark-mcp-assistant ./cmd/spark-mcp-assistant
	go build -o data/hockey-mcp-assistant ./cmd/mcp-hockey-assistant

run-all:
	$(MAKE) run-mcp &
	$(MAKE) run-hockey &
	$(MAKE) run-spark

run-spark:
	go run ./cmd/spark/ matrix

run-hockey:
	go run ./cmd/mcp-hockey-assistant/

run-mcp:
	go run ./cmd/spark-mcp-assistant/

build-docker: build-docker-spark build-docker-mcp

build-docker-spark:
	docker build --target spark -t spark-personal-assistant --pull .

build-docker-mcp:
	docker build --target spark-mcp-assistant -t spark-mcp-assistant --pull .

run-docker: build-docker
	-docker rm -v -f spark-mcp-assistant
	docker run -d --rm --name spark-mcp-assistant --network host -v $(PWD)/mcp-config.yaml:/mcp-config.yaml -v ./data:/data -w / spark-mcp-assistant
	docker run --rm --name spark-personal-assistant --network host -v $(PWD)/spark.yaml:/spark.yaml -v ./data:/data -w / spark-personal-assistant matrix
