DRIVER_NAME=nexentastor-csi-driver
IMAGE_NAME=$(DRIVER_NAME)
DOCKER_FILE=Dockerfile
REGISTRY=nexenta
REGISTRY_LOCAL=10.3.199.92:5000
VERSION ?= $(shell git rev-parse --abbrev-ref HEAD)
COMMIT ?= $(shell git rev-parse HEAD | cut -c 1-7)
DATETIME ?= $(shell date +'%F_%T')
LDFLAGS ?= \
	-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Version=${VERSION} \
	-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Commit=${COMMIT} \
	-X github.com/Nexenta/nexentastor-csi-driver/src/driver.DateTime=${DATETIME}

.PHONY: all
all: build

.PHONY: test
test: test-unit test-e2e-ns test-e2e-k8s

.PHONY: test-unit
test-unit:
	go test ./tests/unit/rest -v -count 1 &&\
	go test ./tests/unit/config -v -count 1

.PHONY: test-e2e-ns
test-e2e-ns:
	go test ./tests/e2e/ns/provider_test.go -v -count 1 \
		--address="https://10.3.199.254:8443" &&\
	go test ./tests/e2e/ns/resolver_test.go -v -count 1 \
		--address="https://10.3.199.254:8443" &&\
	go test ./tests/e2e/ns/resolver_test.go -v -count 1 \
		--address="https://10.3.199.252:8443,https://10.3.199.253:8443"

.PHONY: test-e2e-k8s
test-e2e-k8s:
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@10.3.199.250" \
		--k8sDeploymentFile="./_configs/driver-master-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-single.yaml" &&\
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@10.3.199.250" \
		--k8sDeploymentFile="./_configs/driver-master-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster.yaml"

.PHONY: build
build:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(DRIVER_NAME) -ldflags "$(LDFLAGS)" ./src

.PHONY: container-build
container-build: build
	docker build -f $(DOCKER_FILE) -t $(IMAGE_NAME) .

.PHONY: container-push
container-push: container-build
	docker tag  $(IMAGE_NAME) $(REGISTRY)/$(IMAGE_NAME):$(VERSION)
	docker push $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

.PHONY: container-push-local
container-push-local: container-build
	docker tag  $(IMAGE_NAME) $(REGISTRY_LOCAL)/$(IMAGE_NAME):$(VERSION)
	docker push $(REGISTRY_LOCAL)/$(IMAGE_NAME):$(VERSION)

.PHONY: clean
clean:
	go clean -r -x
	-rm -rf bin
