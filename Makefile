DRIVER_NAME=nexentastor-csi-driver
IMAGE_NAME=$(DRIVER_NAME)
DOCKER_FILE=Dockerfile
DOCKER_FILE_TESTS=Dockerfile.tests
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
all: test-unit build

.PHONY: build
build:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(DRIVER_NAME) -ldflags "$(LDFLAGS)" ./src

.PHONY: container-build
container-build:
	docker build -f $(DOCKER_FILE) -t $(IMAGE_NAME) .

.PHONY: container-push
container-push-remote:
	docker tag  $(IMAGE_NAME) $(REGISTRY)/$(IMAGE_NAME):$(VERSION)
	docker push $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

.PHONY: container-push-local
container-push-local:
	docker tag  $(IMAGE_NAME) $(REGISTRY_LOCAL)/$(IMAGE_NAME):$(VERSION)
	docker push $(REGISTRY_LOCAL)/$(IMAGE_NAME):$(VERSION)

.PHONY: test
test: test-unit test-e2e-ns

.PHONY: test-local
test-local: test-unit test-e2e-ns test-e2e-k8s-local

.PHONY: test-remote
test-remote: test-unit test-e2e-ns test-e2e-k8s-remote

.PHONY: test-unit
test-unit:
	go test ./tests/unit/arrays -v -count 1 &&\
	go test ./tests/unit/rest -v -count 1 &&\
	go test ./tests/unit/config -v -count 1

.PHONY: test-e2e-ns
test-e2e-ns:
	go test ./tests/e2e/ns/provider_test.go -v -count 1 \
		--address="https://10.3.199.254:8443" &&\
	go test ./tests/e2e/ns/provider_test.go -v -count 1 \
		--address="https://10.3.199.252:8443" &&\
	go test ./tests/e2e/ns/resolver_test.go -v -count 1 \
		--address="https://10.3.199.254:8443" &&\
	go test ./tests/e2e/ns/resolver_test.go -v -count 1 \
		--address="https://10.3.199.252:8443,https://10.3.199.253:8443"

.PHONY: test-e2e-k8s-local
test-e2e-k8s-local:
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@10.3.196.219" \
		--k8sDeploymentFile="./_configs/driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-single.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config-tests" &&\
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@10.3.196.219" \
		--k8sDeploymentFile="./_configs/driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config-tests"

.PHONY: test-e2e-k8s-remote
test-e2e-k8s-remote:
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@10.3.196.219" \
		--k8sDeploymentFile="../../../deploy/kubernetes/master/nexentastor-csi-driver-master.yaml" \
		--k8sSecretFile="./_configs/driver-config-single.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config" &&\
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@10.3.196.219" \
		--k8sDeploymentFile="../../../deploy/kubernetes/master/nexentastor-csi-driver-master.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config"

.PHONY: container-test-local
container-test-local:
	docker build -f $(DOCKER_FILE_TESTS) -t $(IMAGE_NAME)-test .
	docker run -i --rm -v ${HOME}/.ssh:/root/.ssh:ro -e NOCOLORS=${NOCOLORS} $(IMAGE_NAME)-test test-local

.PHONY: container-test-remote
container-test-remote:
	docker build -f $(DOCKER_FILE_TESTS) -t $(IMAGE_NAME)-test .
	docker run -i --rm -v ${HOME}/.ssh:/root/.ssh:ro -e NOCOLORS=${NOCOLORS} $(IMAGE_NAME)-test test-remote

.PHONY: clean
clean:
	go clean -r -x
	-rm -rf bin
