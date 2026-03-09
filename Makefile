# Simple Makefile for building remoteweather Go binary

VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
BINARY = bin/remoteweather

.PHONY: all clean help version bump

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
	@echo '  bump     Increment patch version tag and push'

version:
	@echo "Current version: $(VERSION)"

bump:
	@LATEST=$$(git tag --sort=-v:refname | head -1); \
	if [ -z "$$LATEST" ]; then echo "No existing tags found"; exit 1; fi; \
	MAJOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f1); \
	MINOR=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f2); \
	PATCH=$$(echo "$$LATEST" | sed 's/^v//' | cut -d. -f3); \
	NEW="v$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
	echo "$$LATEST -> $$NEW"; \
	git tag "$$NEW" && git push origin "$$NEW"