PLUGIN_NAME=nexentastor-csi-plugin
IMAGE_NAME=$(PLUGIN_NAME)
DOCKER_FILE=Dockerfile
REGISTRY=nexenta
IMAGE_TAG=latest
VERSION ?= $(shell git rev-parse --abbrev-ref HEAD)
COMMIT ?= $(shell git rev-parse HEAD)
LDFLAGS ?= -X github.com/Nexenta/nexentastor-csi-driver/driver.version=${VERSION} -X github.com/Nexenta/nexentastor-csi-driver/driver.commit=${COMMIT}

.PHONY: all build container-build container-push clean

all: build

build:
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(PLUGIN_NAME) -ldflags "$(LDFLAGS)" ./src

container-build: nfs
	docker build -f $(DOCKER_FILE) -t $(IMAGE_NAME) .

container-push: build-container
	docker tag  $(IMAGE_NAME) $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

clean:
	go clean -r -x
	-rm -rf bin
