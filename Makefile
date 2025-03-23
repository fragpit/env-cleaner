ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BUILD_DIR := ${PWD}/bin

BUILD_VERSION ?= $(shell GIT_DIR=${ROOT_DIR}/.git git describe --tags --exact-match 2>/dev/null || git rev-parse --short=8 HEAD)

DOCKER_REGISTRY ?= example-registry.com
DOCKER_REGISTRY_REPO ?= example-repo
DOCKER_APPLICATION := env-cleaner
DOCKER_IMAGE := $(DOCKER_REGISTRY_REPO)/$(DOCKER_APPLICATION)
DOCKER_VERSION_TAG ?= $(shell echo ${BUILD_VERSION} | tr "+" "_")
DOCKER_VERSIONED_IMAGE:= $(DOCKER_IMAGE):$(DOCKER_VERSION_TAG)

REGISTRY_LATEST_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):latest

IMPORT_PATH := github.com/fragpit/env-cleaner
BUILD_VERSION_ARGS := -X $(IMPORT_PATH)/cmd.version=$(BUILD_VERSION)
LINKER_ARGS ?= -s $(BUILD_VERSION_ARGS) # -s omits symbol table and debug information

################ Build targets ################
.PHONY: build
build: #### Build binaries.
	mkdir -p ${BUILD_DIR}
	go build -o ${BUILD_DIR}/$(DOCKER_APPLICATION) -ldflags "$(LINKER_ARGS)"

################ Test targets ################
.PHONY: lint
lint: #### Lint code.
	@go version
	@golangci-lint version
	${info app version $(BUILD_VERSION)}
	cd ${ROOT_DIR} && \
		golangci-lint run --max-same-issues 0 --max-issues-per-linter 0 --timeout 5m --verbose

################ Container build and publish targets, mainly used by CI ################
.PHONY: image-build
image-build: ### Build container images.
	docker build --progress=plain --file "$(ROOT_DIR)/Dockerfile" --tag "$(DOCKER_VERSIONED_IMAGE)" "$(ROOT_DIR)"
	docker tag "$(DOCKER_VERSIONED_IMAGE)" "$(DOCKER_IMAGE):latest"

.PHONY: image-clean
image-clean: ### Remove last version of the images.
	docker rmi -f "$$(docker images -q $(TAGGED_IMAGE))"
	docker image prune -f --filter label=stage=env-cleaner-$(BUILD_VERSION)

.PHONY: image-tag
image-tag: ### Tag images.
	docker tag "$(DOCKER_VERSIONED_IMAGE)" "$(DOCKER_REGISTRY)/$(DOCKER_VERSIONED_IMAGE)"
	if [ "$(GIT_BRANCH)" = "master" ] || [ "$(CI_COMMIT_REF_NAME)" = "master" ]; then\
		docker tag "$(DOCKER_VERSIONED_IMAGE)" "$(REGISTRY_LATEST_IMAGE)";\
	fi

.PHONY: image-push
image-push: image-tag ### Push images to registry.
	docker push "$(DOCKER_REGISTRY)/$(DOCKER_VERSIONED_IMAGE)"
	echo "$(DOCKER_REGISTRY)/$(DOCKER_VERSIONED_IMAGE)" > published_images.txt
	if [ "$(GIT_BRANCH)" = "master" ] || [ "$(CI_COMMIT_REF_NAME)" = "master" ]; then\
		docker push "$(REGISTRY_LATEST_IMAGE)";\
		echo "$(REGISTRY_LATEST_IMAGE)" >> published_images.txt;\
	fi

.PHONY: help
help: ##### Show this help.
	@sed -e '/__hidethis__/d; /###/!d; s/:.\+#### /\t\t/g; s/:.\+#### /\t\t\t/g; s/:.\+### /\t/g' $(MAKEFILE_LIST)
