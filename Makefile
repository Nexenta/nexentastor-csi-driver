# NexentaStor CSI Driver makefile
#
# Test options to be set before run tests:
# NOCOLOR=1                # disable colors
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

GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD | sed -e "s/.*\\///")
GIT_TAG = $(shell git describe --tags)
# use git branch as default version if not set by env variable, if HEAD is detached that use the most recent tag
VERSION ?= $(if $(subst HEAD,,${GIT_BRANCH}),$(GIT_BRANCH),$(GIT_TAG))
COMMIT ?= $(shell git rev-parse HEAD | cut -c 1-7)
DATETIME ?= $(shell date +'%F_%T')
LDFLAGS ?= \
	-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.Version=${VERSION} \
	-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.Commit=${COMMIT} \
	-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.DateTime=${DATETIME}

.PHONY: all
all:
	@echo "Some of commands:"
	@echo "  container-build                 - build driver container"
	@echo "  container-push-local            - push driver to local registry (${REGISTRY_DEVELOPMENT})"
	@echo "  container-push-remote           - push driver to hub.docker.com registry (${REGISTRY_PRODUCTION})"
	@echo "  test-all-local-image-container  - run all test using driver from local registry"
	@echo "  test-all-remote-image-container - run all test using driver from hub.docker.com"
	@echo "  release                         - create and publish a new release"
	@echo ""
	@make print-variables

.PHONY: print-variables
print-variables:
	@echo "Variables:"
	@echo "  VERSION:    ${VERSION}"
	@echo "  GIT_BRANCH: ${GIT_BRANCH}"
	@echo "  GIT_TAG:    ${GIT_TAG}"
	@echo "  COMMIT:     ${COMMIT}"
	@echo "Testing variables:"
	@echo "  TEST_K8S_IP: ${TEST_K8S_IP}"

.PHONY: vet
vet:
	CGO_ENABLED=0 go vet -v ./...

.PHONY: fmt
fmt:
	CGO_ENABLED=0 go fmt ./...

.PHONY: build
build: vet fmt
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/${DRIVER_NAME} -ldflags "${LDFLAGS}" ./cmd

.PHONY: container-build
container-build:
ifeq (${VERSION}, master)
	docker build -f ${DOCKER_FILE} -t ${IMAGE_NAME}:${VERSION} --build-arg VERSION=${VERSION} .
else
	docker build -f ${DOCKER_FILE} -t ${IMAGE_NAME}:v${VERSION} --build-arg VERSION=v${VERSION} .
endif

.PHONY: container-push-local
container-push-local:
ifeq (${VERSION}, master)
	docker build -f ${DOCKER_FILE} -t ${IMAGE_NAME}:${VERSION} --build-arg VERSION=${VERSION} .
	docker tag  ${IMAGE_NAME}:${VERSION} ${REGISTRY_LOCAL}/${IMAGE_NAME}:${VERSION}
	docker push ${REGISTRY_LOCAL}/${IMAGE_NAME}:${VERSION}
else
	docker build -f ${DOCKER_FILE} -t ${IMAGE_NAME}:v${VERSION} --build-arg VERSION=v${VERSION} .
	docker tag  ${IMAGE_NAME}:v${VERSION} ${REGISTRY_LOCAL}/${IMAGE_NAME}:v${VERSION}
	docker push ${REGISTRY_LOCAL}/${IMAGE_NAME}:v${VERSION}
endif

.PHONY: container-push-remote
container-push-remote:
ifeq (${VERSION}, master)
	docker build -f ${DOCKER_FILE} -t ${IMAGE_NAME}:${VERSION} --build-arg VERSION=${VERSION} .
	docker tag  ${IMAGE_NAME}:${VERSION} ${REGISTRY}/${IMAGE_NAME}:${VERSION}
	docker push ${REGISTRY}/${IMAGE_NAME}:${VERSION}
else
	docker build -f ${DOCKER_FILE} -t ${IMAGE_NAME}:v${VERSION} --build-arg VERSION=v${VERSION} .
	docker tag  ${IMAGE_NAME}:v${VERSION} ${REGISTRY}/${IMAGE_NAME}:v${VERSION}
	docker push ${REGISTRY}/${IMAGE_NAME}:v${VERSION}
endif

.PHONY: test
test: test-unit

.PHONY: test-unit
test-unit:
	go test ./tests/unit/arrays -v -count 1
	go test ./tests/unit/config -v -count 1
.PHONY: test-unit-container
test-unit-container:
	docker build -f ${DOCKER_FILE_TESTS} -t ${IMAGE_NAME}-test --build-arg VERSION=${VERSION} .
	docker run -i --rm -e NOCOLORS=${NOCOLORS} ${IMAGE_NAME}-test test-unit

# run e2e k8s tests using image from local docker registry
.PHONY: test-e2e-k8s-local-image
test-e2e-k8s-local-image: check-env-TEST_K8S_IP
	sed -e "s/image: nexenta/image: ${REGISTRY_LOCAL}/g" \
		./deploy/kubernetes/nexentastor-csi-driver.yaml > /tmp/nexentastor-csi-driver-local.yaml
	go test -timeout 30m tests/e2e/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_K8S_IP}" \
		--k8sDeploymentFile="/tmp/nexentastor-csi-driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-single-default.yaml" \
		--fsTypeFlag="nfs"
	go test -timeout 30m tests/e2e/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_K8S_IP}" \
		--k8sDeploymentFile="/tmp/nexentastor-csi-driver-local.yaml" \
		--k8sSecretFile="./_configs/driver-config-single-cifs.yaml"
.PHONY: test-e2e-k8s-local-image-container
test-e2e-k8s-local-image-container: check-env-TEST_K8S_IP
	docker build -f ${DOCKER_FILE_TESTS} -t ${IMAGE_NAME}-test --build-arg VERSION=${VERSION} \
	--build-arg TESTRAIL_URL=${TESTRAIL_URL} \
	--build-arg TESTRAIL_USR=${TESTRAIL_USR} \
	--build-arg TESTRAIL_PSWD=${TESTRAIL_PSWD} .
	docker run -i --rm -v ${HOME}/.ssh:/root/.ssh:ro \
		-e NOCOLORS=${NOCOLORS} -e TEST_K8S_IP=${TEST_K8S_IP} \
		${IMAGE_NAME}-test test-e2e-k8s-local-image

# run e2e k8s tests using image from hub.docker.com
.PHONY: test-e2e-k8s-remote-image
test-e2e-k8s-remote-image: check-env-TEST_K8S_IP
	go test -timeout 30m tests/e2e/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_K8S_IP}" \
		--k8sDeploymentFile="../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
		--k8sSecretFile="./_configs/driver-config-single-default.yaml" \
		--fsTypeFlag="nfs"
	go test -timeout 30m tests/e2e/driver_test.go -v -count 1 \
		--k8sConnectionString="root@${TEST_K8S_IP}" \
		--k8sDeploymentFile="../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
		--k8sSecretFile="./_configs/driver-config-single-cifs.yaml"
.PHONY: test-e2e-k8s-local-image-container
test-e2e-k8s-remote-image-container: check-env-TEST_K8S_IP
	docker build -f ${DOCKER_FILE_TESTS} -t ${IMAGE_NAME}-test --build-arg VERSION=${VERSION} \
	--build-arg TESTRAIL_URL=${TESTRAIL_URL} \
	--build-arg TESTRAIL_USR=${TESTRAIL_USR} \
	--build-arg TESTRAIL_PSWD=${TESTRAIL_PSWD} .
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
	docker build \
		--build-arg CSI_SANITY_VERSION_TAG=v2.1.0 \
		-f ${DOCKER_FILE_TEST_CSI_SANITY} \
		-t ${IMAGE_NAME}-test-csi-sanity .
	docker run --privileged=true -i -e NOCOLORS=${NOCOLORS} ${IMAGE_NAME}-test-csi-sanity
	docker image prune -f
	docker images | grep nexentastor-csi-driver-test-csi-sanity | awk '{print $$1}' | xargs docker rmi -f

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

.PHONY: release
release:
	@echo "New tag: 'v${VERSION}'\n\n \
		To change version set enviroment variable 'VERSION=X.X.X make release'.\n\n \
		Confirm that:\n \
		1. New version will be based on current '${GIT_BRANCH}' git branch\n \
		2. Driver container '${IMAGE_NAME}' will be built\n \
		3. Login to hub.docker.com will be requested\n \
		4. Driver version '${REGISTRY}/${IMAGE_NAME}:v${VERSION}' will be pushed to hub.docker.com\n \
		5. CHANGELOG.md file will be updated\n \
		6. Git tag 'v${VERSION}' will be created and pushed to the repository.\n\n \
		Are you sure? [y/N]: "
	@(read ANSWER && case "$$ANSWER" in [yY]) true;; *) false;; esac)
	git checkout -b ${VERSION}
	sed -i 's/:master/:v$(VERSION)/g' deploy/kubernetes/nexentastor-csi-driver.yaml
	docker login
	make generate-changelog
	make container-build
	make container-push-remote
	git add CHANGELOG.md
	git add deploy/kubernetes/nexentastor-csi-driver.yaml
	git commit -m "release v${VERSION}"
	git push origin ${VERSION}
	git tag v${VERSION}
	git push --tags

.PHONY: generate-changelog
generate-changelog:
	@echo "Release tag: ${VERSION}\n"
	docker build -f ${DOCKER_FILE_PRE_RELEASE} -t ${DOCKER_IMAGE_PRE_RELEASE} --build-arg VERSION=${VERSION} .
	-docker rm -f ${DOCKER_CONTAINER_PRE_RELEASE}
	docker create --name ${DOCKER_CONTAINER_PRE_RELEASE} ${DOCKER_IMAGE_PRE_RELEASE}
	docker cp \
		${DOCKER_CONTAINER_PRE_RELEASE}:/go/src/github.com/Nexenta/nexentastor-csi-driver/CHANGELOG.md \
		./CHANGELOG.md
	docker rm ${DOCKER_CONTAINER_PRE_RELEASE}

.PHONY: clean
clean:
	-go clean -r -x
	-rm -rf bin
