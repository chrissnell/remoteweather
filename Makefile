# Simple Makefile for building remoteweather Go binary

VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
BINARY = bin/remoteweather

.PHONY: all clean help version

all: $(BINARY)

$(BINARY):
	go build -ldflags "-X 'github.com/chrissnell/remoteweather/internal/constants.Version=$(VERSION)' -X 'github.com/chrissnell/remoteweather/internal/constants.CommitID=$(COMMIT)'" -o $(BINARY) ./cmd/remoteweather

clean:
	rm -f $(BINARY)

help:
	@echo 'Usage: make [target] ...'
	@echo ''
	@echo 'Targets:'
	@echo '  all      Build the remoteweather binary (default)'
	@echo '  clean    Remove built binaries'
	@echo '  help     Show this help message'
	@echo '  version  Show current version'

version:
	@echo "Current version: $(VERSION)" 