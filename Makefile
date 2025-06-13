# Docker image configuration
DOCKER_REGISTRY ?= docker.io
DOCKER_REPO ?= $(DOCKER_REGISTRY)/chrissnell/remoteweather
VERSION ?= $(shell git describe --tags --always --dirty)
IMAGE_NAME = $(DOCKER_REPO)
PLATFORM = linux/amd64

# Build flags
DOCKER_BUILD_FLAGS = --platform $(PLATFORM)

.PHONY: help build tag push clean login

help: ## Show this help message
	@echo 'Usage: make [target] ...'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Docker image for linux/amd64
	docker build $(DOCKER_BUILD_FLAGS) -t $(IMAGE_NAME):latest .

tag: build ## Tag the Docker image with version and latest
	docker tag $(IMAGE_NAME):latest $(IMAGE_NAME):$(VERSION)

push: tag ## Push the Docker image to Docker Hub
	docker push $(IMAGE_NAME):latest
	docker push $(IMAGE_NAME):$(VERSION)

clean: ## Remove local Docker images
	docker rmi $(IMAGE_NAME):latest || true
	docker rmi $(IMAGE_NAME):$(VERSION) || true

version: ## Display the current version that would be built
	@echo "Current version: $(VERSION)"

login: ## Log in to Docker Hub
	docker login $(DOCKER_REGISTRY)

# Example usage:
# make build      - Build the image
# make push       - Build, tag, and push to Docker Hub
# make version    - Show current version 