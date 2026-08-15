.PHONY: build test install site site-serve
build:
	CGO_ENABLED=0 go build -o bin/zook .
test:
	go test ./...
install: build
	install -m 0755 bin/zook /usr/local/bin/zook
site:
	go run ./site/gen -out site/dist
site-serve:
	go run ./site/gen -out site/dist -serve
