.PHONY: run test build
run:
	go run ./cmd/server
test:
	go test -race ./...
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/training ./cmd/server
