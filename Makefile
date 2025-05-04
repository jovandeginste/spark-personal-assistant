all: test lint
test:
	go test -short -count 1 -mod vendor -covermode=atomic -gcflags=all=-l ./...

lint:
	golangci-lint run --allow-parallel-runners --fix --config=./.golangci.yml --color=always
