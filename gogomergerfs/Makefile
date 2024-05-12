APP ?= gogomergerfs

VERSION ?= $(shell git rev-parse --abbrev-ref HEAD)
MERGERFS_VERSION ?= 2.37.1
GO_VERSION ?= $(shell cat $(CURDIR)/.go-version | head -n1)

REGISTRY ?= docker.io/on2e
TAG ?= $(VERSION)-mergerfs$(MERGERFS_VERSION)
IMAGE ?= $(REGISTRY)/$(APP):$(TAG)

.PHONY: all
all: gogomergerfs

.PHONY: gogomergerfs
gogomergerfs:
	@mkdir -p bin
	CGO_ENABLED=0 go build \
		-ldflags "-s -w -extldflags=-static" \
		-o bin/gogomergerfs \
		./cmd/gogomergerfs

# TODO: try docker buildx
.PHONY: docker-build
docker-build:
	docker build \
		--tag $(IMAGE) \
		--build-arg MERGERFS_VERSION=$(MERGERFS_VERSION) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--file Dockerfile \
		.

.PHONY: docker-push
docker-push: docker-build
	docker push $(IMAGE)

.PHONY: clean
clean:
	rm -rf bin
