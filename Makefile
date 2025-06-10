all: test lint
test:
	CGO_ENABLED=1 go test -short -count 1 -tags goolm -mod vendor -covermode=atomic -gcflags=all=-l ./...

lint:
	CGO_ENABLED=1 golangci-lint run --allow-parallel-runners --fix --config=./.golangci.yml --color=always --build-tags=goolm

build-docker:
	docker build -t spark-personal-assistant --pull .
