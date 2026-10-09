.PHONY: build build-dev build-platform install dist test clean

GO ?= go
SWIFT ?= swift
BINARY := bin/audio-record
MACOS_PACKAGE := platforms/macos
MACOS_HELPER := $(MACOS_PACKAGE)/.build/release/audio-capture-macos
EMBEDDED_HELPER := internal/recorder/assets/audio-capture-macos
PREFIX ?= $(HOME)/.local

build: build-platform
	mkdir -p bin
	$(GO) build -tags embedded -trimpath -ldflags "-s -w" -o $(BINARY) ./cmd/audio-record

build-dev:
	mkdir -p bin
	$(GO) build -o $(BINARY) ./cmd/audio-record

ifeq ($(shell uname -s),Darwin)
build-platform:
	$(SWIFT) build --package-path $(MACOS_PACKAGE) --configuration release
	mkdir -p $(dir $(EMBEDDED_HELPER))
	cp $(MACOS_HELPER) $(EMBEDDED_HELPER)
else
build-platform:
	@echo "No native capture helper is available for this platform yet."
endif

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 0755 $(BINARY) $(DESTDIR)$(PREFIX)/bin/audio-record
	@echo "Installed $(DESTDIR)$(PREFIX)/bin/audio-record"

dist: build
	mkdir -p dist
	tar -czf dist/audio-record-darwin-$(shell $(GO) env GOARCH).tar.gz -C bin audio-record
	cd dist && shasum -a 256 audio-record-darwin-$(shell $(GO) env GOARCH).tar.gz > audio-record-darwin-$(shell $(GO) env GOARCH).tar.gz.sha256

test:
	$(GO) test ./...
ifeq ($(shell uname -s),Darwin)
	$(SWIFT) build --package-path $(MACOS_PACKAGE)
endif

clean:
	rm -rf bin dist platforms/macos/.build
	rm -f $(EMBEDDED_HELPER)
