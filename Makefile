VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
BINARY := bin/grove

.PHONY: build install test lint clean release-dry

build:
	go build $(LDFLAGS) -o $(BINARY) .

install: build
	cp $(BINARY) /usr/local/bin/

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin/ dist/

release-dry:
	goreleaser release --snapshot --clean
