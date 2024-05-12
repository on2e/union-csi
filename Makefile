APP ?= union-csi-driver

VERSION ?= $(shell git rev-parse --abbrev-ref HEAD)
GO_VERSION ?= $(shell cat $(CURDIR)/.go-version | head -n1)

REGISTRY ?= docker.io/on2e
TAG ?= $(VERSION)
IMAGE ?= $(REGISTRY)/$(APP):$(TAG)

.PHONY: all
all: union-csi-driver

.PHONY: union-csi-driver
union-csi-driver:
	@mkdir -p bin
	CGO_ENABLED=0 go build \
		-ldflags "-X github.com/on2e/union-csi-driver/pkg/csi/driver.driverVersion=$(VERSION) -extldflags=-static" \
		-o bin/union-csi-driver \
		./cmd/union-csi-driver

# TODO: try docker buildx
.PHONY: docker-build
docker-build:
	docker build --tag $(IMAGE) --file Dockerfile .

.PHONY: docker-push
docker-push: docker-build
	docker push $(IMAGE)

.PHONY: clean
clean:
	rm -rf bin
