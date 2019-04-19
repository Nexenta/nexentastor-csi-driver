# NexentaStor CSI Driver makefile
#
# Test options to be set before run tests:
# - NOCOLOR=1                # disable colors
# - TEST_K8S_IP=10.3.199.250 # e2e k8s tests
#

DRIVER_NAME = nexentastor-csi-driver
IMAGE_NAME ?= ${DRIVER_NAME}

DOCKER_FILE = Dockerfile
DOCKER_FILE_TESTS = Dockerfile.tests
DOCKER_FILE_TEST_CSI_SANITY = Dockerfile.csi-sanity
DOCKER_FILE_PRE_RELEASE = Dockerfile.pre-release
DOCKER_IMAGE_PRE_RELEASE = nexentastor-csi-driver-pre-release
DOCKER_CONTAINER_PRE_RELEASE = ${DOCKER_IMAGE_PRE_RELEASE}-container

REGISTRY ?= nexenta
REGISTRY_LOCAL ?= 10.3.199.92:5000

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
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/${DRIVER_NAME} -ldflags "${LDFLAGS}" ./cmd

.PHONY: container-build
container-build:
	docker build -f ${DOCKER_FILE} -t ${IMAGE_NAME} .

.PHONY: container-push-local
container-push-local:
	docker tag  ${IMAGE_NAME} ${REGISTRY_LOCAL}/${IMAGE_NAME}:${VERSION}
	docker push ${REGISTRY_LOCAL}/${IMAGE_NAME}:${VERSION}

.PHONY: container-push-remote
container-push-remote:
	docker tag  ${IMAGE_NAME} ${REGISTRY}/${IMAGE_NAME}:${VERSION}
	docker push ${REGISTRY}/${IMAGE_NAME}:${VERSION}

.PHONY: test
test: test-unit

.PHONY: test-unit
test-unit:
	go test ./tests/unit/arrays -v -count 1
	go test ./tests/unit/config -v -count 1
.PHONY: test-unit-container
test-unit-container:
	docker build -f ${DOCKER_FILE_TESTS} -t ${IMAGE_NAME}-test .
	docker run -i --rm -e NOCOLORS=${NOCOLORS} ${IMAGE_NAME}-test test-unit

# run e2e k8s tests using image from local docker registry
.PHONY: test-e2e-k8s-local-image
test-e2e-k8s-local-image: check-env-TEST_K8S_IP
	go test tests/e2e/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_K8S_IP}" \
		--k8sDeploymentFile="./_configs/driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster-default.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config-tests"
	go test tests/e2e/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_K8S_IP}" \
		--k8sDeploymentFile="./_configs/driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster-cifs.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config-tests"
	# go test tests/e2e/driver_test.go -v -count 1 \
	# 	--k8sConnectionString="root@${TEST_K8S_IP}" \
	# 	--k8sDeploymentFile="./_configs/driver-local.yaml" \
	# 	--k8sSecretFile="./_configs/driver-config-single.yaml" \
	# 	--k8sSecretName="nexentastor-csi-driver-config-tests"
.PHONY: test-e2e-k8s-local-image-container
test-e2e-k8s-local-image-container: check-env-TEST_K8S_IP
	docker build -f ${DOCKER_FILE_TESTS} -t ${IMAGE_NAME}-test .
	docker run -i --rm -v ${HOME}/.ssh:/root/.ssh:ro \
		-e NOCOLORS=${NOCOLORS} -e TEST_K8S_IP=${TEST_K8S_IP} \
		${IMAGE_NAME}-test test-e2e-k8s-local-image

# run e2e k8s tests using image from hub.docker.com
.PHONY: test-e2e-k8s-remote-image
test-e2e-k8s-remote-image: check-env-TEST_K8S_IP
	go test tests/e2e/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_K8S_IP}" \
		--k8sDeploymentFile="../../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster-default.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config"
	go test tests/e2e/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_K8S_IP}" \
		--k8sDeploymentFile="../../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
		--k8sSecretFile="./_configs/driver-config-cluster-cifs.yaml" \
		--k8sSecretName="nexentastor-csi-driver-config"
	# go test tests/e2e/driver_test.go -v -count 1 \
	# 	--k8sConnectionString="root@${TEST_K8S_IP}" \
	# 	--k8sDeploymentFile="../../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
	# 	--k8sSecretFile="./_configs/driver-config-single.yaml" \
	# 	--k8sSecretName="nexentastor-csi-driver-config"
.PHONY: test-e2e-k8s-local-image-container
test-e2e-k8s-remote-image-container: check-env-TEST_K8S_IP
	docker build -f ${DOCKER_FILE_TESTS} -t ${IMAGE_NAME}-test .
	docker run -i --rm -v ${HOME}/.ssh:/root/.ssh:ro \
		-e NOCOLORS=${NOCOLORS} -e TEST_K8S_IP=${TEST_K8S_IP} \
		${IMAGE_NAME}-test test-e2e-k8s-remote-image

# csi-sanity tests:
# - tests make requests to actual NS, config file: ./tests/csi-sanity/*.yaml
# - create container with driver and csi-sanity (https://github.com/kubernetes-csi/csi-test)
# - run container to execute tests
# - nfs client requires running container as privileged one
.PHONY: test-csi-sanity-container
test-csi-sanity-container:
	docker build -f ${DOCKER_FILE_TEST_CSI_SANITY} -t ${IMAGE_NAME}-test-csi-sanity .
	docker run --privileged=true -i --rm -e NOCOLORS=${NOCOLORS} ${IMAGE_NAME}-test-csi-sanity

# run all tests (local registry image)
.PHONY: test-all-local-image
test-all-local-image: \
	test-unit \
	test-e2e-k8s-local-image
.PHONY: test-all-local-image-container
test-all-local-image-container: \
	test-unit-container \
	test-csi-sanity-container \
	test-e2e-k8s-local-image-container

# run all tests (hub.github.com image)
.PHONY: test-all-remote-image
test-all-remote-image: \
	test-unit \
	test-e2e-k8s-remote-image
.PHONY: test-all-remote-image-container
test-all-remote-image-container: \
	test-unit-container \
	test-csi-sanity-container \
	test-e2e-k8s-remote-image-container

.PHONY: check-env-TEST_K8S_IP
check-env-TEST_K8S_IP:
ifeq ($(strip ${TEST_K8S_IP}),)
	$(error "Error: environment variable TEST_K8S_IP is not set (e.i. 10.3.199.250)")
endif

.PHONY: pre-release-container
pre-release-container: check-env-NEXT_TAG
	@echo "Next release tag: ${NEXT_TAG}\n"
	docker build -f ${DOCKER_FILE_PRE_RELEASE} -t ${DOCKER_IMAGE_PRE_RELEASE} --build-arg NEXT_TAG=${NEXT_TAG} .
	-docker rm -f ${DOCKER_CONTAINER_PRE_RELEASE}
	docker create --name ${DOCKER_CONTAINER_PRE_RELEASE} ${DOCKER_IMAGE_PRE_RELEASE}
	docker cp \
		${DOCKER_CONTAINER_PRE_RELEASE}:/go/src/github.com/Nexenta/nexentastor-csi-driver/CHANGELOG.md \
		./CHANGELOG.md
	docker rm ${DOCKER_CONTAINER_PRE_RELEASE}

.PHONY: check-env-NEXT_TAG
check-env-NEXT_TAG:
ifeq ($(strip ${NEXT_TAG}),)
	$(error "Error: environment variable NEXT_TAG is not set (e.i. '1.2.3')")
endif

.PHONY: clean
clean:
	-go clean -r -x
	-rm -rf bin
