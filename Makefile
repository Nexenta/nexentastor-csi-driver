DRIVER_NAME=nexentastor-csi-plugin
IMAGE_NAME=$(DRIVER_NAME)
DOCKER_FILE=Dockerfile
REGISTRY=10.3.199.92:5000
IMAGE_TAG=latest
VERSION ?= $(shell git rev-parse --abbrev-ref HEAD)
COMMIT ?= $(shell git rev-parse HEAD | cut -c 1-7)
DATETIME ?= $(shell date +'%F_%T')
LDFLAGS ?= \
	-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Version=${VERSION} \
	-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Commit=${COMMIT} \
	-X github.com/Nexenta/nexentastor-csi-driver/src/driver.DateTime=${DATETIME}

.PHONY: all test build container-build container-push clean

all: build

test:
	go test ./tests/* -v -count 1

build:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(DRIVER_NAME) -ldflags "$(LDFLAGS)" ./src

container-build: build
	docker build -f $(DOCKER_FILE) -t $(IMAGE_NAME) .

container-push: container-build
	docker tag  $(IMAGE_NAME) $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

clean:
	go clean -r -x
	-rm -rf bin
