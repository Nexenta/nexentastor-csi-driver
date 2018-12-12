DRIVER_NAME=nexentastor-csi-driver
IMAGE_NAME=$(DRIVER_NAME)
DOCKER_FILE=Dockerfile
DOCKER_FILE_TESTS=Dockerfile.tests
DOCKER_FILE_TEST_CSI_SANITY=Dockerfile.csi-sanity
REGISTRY=nexenta
REGISTRY_LOCAL=10.3.199.92:5000
TEST_MACHINE_IP=10.3.199.250
VERSION ?= $(shell git rev-parse --abbrev-ref HEAD | sed -e "s/.*\\///")
COMMIT ?= $(shell git rev-parse HEAD | cut -c 1-7)
DATETIME ?= $(shell date +'%F_%T')
LDFLAGS ?= \
	-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.Version=${VERSION} \
	-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.Commit=${COMMIT} \
	-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.DateTime=${DATETIME}

.PHONY: all
all: test build

.PHONY: build
build:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(DRIVER_NAME) -ldflags "$(LDFLAGS)" ./cmd

.PHONY: container-build
container-build:
	docker build -f $(DOCKER_FILE) -t $(IMAGE_NAME) .

.PHONY: container-push-local
container-push-local:
	docker tag  $(IMAGE_NAME) $(REGISTRY_LOCAL)/$(IMAGE_NAME):$(VERSION)
	docker push $(REGISTRY_LOCAL)/$(IMAGE_NAME):$(VERSION)

.PHONY: container-push-remote
container-push-remote:
	docker tag  $(IMAGE_NAME) $(REGISTRY)/$(IMAGE_NAME):$(VERSION)
	docker push $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

.PHONY: test
test: test-unit

.PHONY: test-unit
test-unit:
	go test ./tests/unit/arrays -v -count 1 &&\
	go test ./tests/unit/config -v -count 1 &&\
	go test ./tests/unit/rest -v -count 1 &&\
	go test ./tests/unit/ns -v -count 1
.PHONY: test-unit-container
test-unit-container:
	docker build -f $(DOCKER_FILE_TESTS) -t $(IMAGE_NAME)-test .
	docker run -i --rm -e NOCOLORS=${NOCOLORS} $(IMAGE_NAME)-test test-unit

.PHONY: test-e2e-ns
test-e2e-ns:
	go test ./tests/e2e/ns/provider/provider_test.go -v -count 1 \
		--address="https://10.3.199.254:8443" &&\
	go test ./tests/e2e/ns/provider/provider_test.go -v -count 1 \
		--address="https://10.3.199.252:8443" \
		--cluster=true &&\
	go test ./tests/e2e/ns/resolver/resolver_test.go -v -count 1 \
		--address="https://10.3.199.254:8443" &&\
	go test ./tests/e2e/ns/resolver/resolver_test.go -v -count 1 \
		--address="https://10.3.199.252:8443,https://10.3.199.253:8443"
.PHONY: test-e2e-ns-container
test-e2e-ns-container:
	docker build -f $(DOCKER_FILE_TESTS) -t $(IMAGE_NAME)-test .
	docker run -i --rm -v ${HOME}/.ssh:/root/.ssh:ro -e NOCOLORS=${NOCOLORS} $(IMAGE_NAME)-test test-e2e-ns

# run e2e k8s tests using image from local docker registry
.PHONY: test-e2e-k8s-local-image
test-e2e-k8s-local-image:
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_MACHINE_IP}" \
		--k8sDeploymentFile="./_configs/driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-single.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config-tests" &&\
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_MACHINE_IP}" \
		--k8sDeploymentFile="./_configs/driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster-default.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config-tests" &&\
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_MACHINE_IP}" \
		--k8sDeploymentFile="./_configs/driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster-cifs.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config-tests"
.PHONY: test-e2e-k8s-local-image-container
test-e2e-k8s-local-image-container:
	docker build -f $(DOCKER_FILE_TESTS) -t $(IMAGE_NAME)-test .
	docker run -i --rm -v ${HOME}/.ssh:/root/.ssh:ro -e NOCOLORS=${NOCOLORS} $(IMAGE_NAME)-test test-e2e-k8s-local-image

# run e2e k8s tests using image from hub.docker.com
.PHONY: test-e2e-k8s-remote-image
test-e2e-k8s-remote-image:
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_MACHINE_IP}" \
		--k8sDeploymentFile="../../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
		--k8sSecretFile="./_configs/driver-config-single.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config" &&\
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_MACHINE_IP}" \
		--k8sDeploymentFile="../../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster-default.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config" &&\
	go test tests/e2e/driver/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_MACHINE_IP}" \
		--k8sDeploymentFile="../../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster-cifs.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config"
.PHONY: test-e2e-k8s-local-image-container
test-e2e-k8s-remote-image-container:
	docker build -f $(DOCKER_FILE_TESTS) -t $(IMAGE_NAME)-test .
	docker run -i --rm -v ${HOME}/.ssh:/root/.ssh:ro -e NOCOLORS=${NOCOLORS} $(IMAGE_NAME)-test test-e2e-k8s-remote-image

# csi-sanity tests:
# - tests make requests to actual NS, config file: ./tests/csi-sanity/*.yaml
# - create container with driver and csi-sanity (https://github.com/kubernetes-csi/csi-test)
# - run container to execute tests
# - nfs client requires running container as privileged one
.PHONY: test-csi-sanity-container
test-csi-sanity-container:
	docker build -f $(DOCKER_FILE_TEST_CSI_SANITY) -t $(IMAGE_NAME)-test-csi-sanity .
	docker run --privileged=true -i --rm -e NOCOLORS=${NOCOLORS} $(IMAGE_NAME)-test-csi-sanity

# run all tests (local registry image)
.PHONY: test-all-local-image
test-all-local-image: test-unit test-e2e-ns test-e2e-k8s-local-image
.PHONY: test-all-local-image-container
test-all-local-image-container: test-unit-container test-e2e-ns-container test-csi-sanity-container test-e2e-k8s-local-image-container

# run all tests (hub.github.com image)
.PHONY: test-all-remote-image
test-all-remote-image: test-unit test-e2e-ns test-e2e-k8s-remote-image
.PHONY: test-all-remote-image-container
test-all-remote-image-container: test-unit-container test-e2e-ns-container test-csi-sanity-container test-e2e-k8s-remote-image-container

.PHONY: clean
clean:
	go clean -r -x
	-rm -rf bin
