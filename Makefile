.PHONY: build test install
build:
	CGO_ENABLED=0 go build -o bin/zook .
test:
	go test ./...
install: build
	install -m 0755 bin/zook /usr/local/bin/zook
