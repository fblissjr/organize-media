VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build test clean synology-amd64 synology-arm64

build:
	go build -ldflags "-X main.version=$(VERSION)" -o organize-media .

test:
	go test -v -count=1 ./...

clean:
	rm -f organize-media

synology-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o organize-media-linux-amd64 .

synology-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o organize-media-linux-arm64 .
