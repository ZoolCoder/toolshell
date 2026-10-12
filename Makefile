.PHONY: all test lint fmt cover
all: lint test
test:
	go test ./...
lint:
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
fmt:
	gofmt -w .
cover:
	go test -coverpkg=./... -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1
