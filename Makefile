.PHONY: build test fmt lint install clean release

BINARY := mihoro
BIN_DIR := $(HOME)/.local/bin
VERSION := $(shell grep 'Version = ' internal/version/version.go | head -1 | sed 's/.*"\(.*\)".*/\1/')
ifeq ($(CI),true)
  COMMIT := $(shell git rev-parse --short HEAD)
else
  COMMIT := dev
endif
LDFLAGS := -ldflags="-s -w \
	-X mihoro-go/internal/version.Version=$(VERSION) \
	-X mihoro-go/internal/version.Commit=$(COMMIT)"

build:
	mkdir -p dist
	CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY) ./cmd/mihoro

test:
	go test -v -count=1 ./...

fmt:
	go fmt ./...

lint:
	go vet ./...

install: build
	mkdir -p $(BIN_DIR)
	cp dist/$(BINARY) $(BIN_DIR)/$(BINARY)
	@echo "Installed $(BINARY) to $(BIN_DIR)/$(BINARY)"

# Cross-compile for all supported Linux architectures
release:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/mihoro
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/mihoro
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-linux-armv7 ./cmd/mihoro
	GOOS=linux GOARCH=riscv64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-linux-riscv64 ./cmd/mihoro
	@echo "Release $(VERSION) ($(COMMIT)) built in dist/"

clean:
	rm -f $(BINARY)
	rm -rf dist/
