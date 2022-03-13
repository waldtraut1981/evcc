.PHONY: default all clean install install-ui ui assets lint test-ui lint-ui test build test-release release
.PHONY: docker publish-testing publish-latest publish-nightly publish-release
.PHONY: prepare-image image-rootfs image-update
.PHONY: soc stamps

# build vars
TAG_NAME := $(shell test -d .git && git describe --abbrev=0 --tags)
SHA := $(shell test -d .git && git rev-parse --short HEAD)
COMMIT := $(SHA)
# hide commit for releases
ifeq ($(RELEASE),1)
    COMMIT :=
endif
VERSION := $(if $(TAG_NAME),$(TAG_NAME),$(SHA))
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
BUILD_TAGS := -tags=release,viper_yaml3
LD_FLAGS := -X github.com/evcc-io/evcc/server.Version=$(VERSION) -X github.com/evcc-io/evcc/server.Commit=$(COMMIT) -s -w
BUILD_ARGS := -ldflags='$(LD_FLAGS)'

# docker
DOCKER_IMAGE := andig/evcc
ALPINE_VERSION := 3.15
TARGETS := arm.v6,arm.v8,amd64

# gokrazy image
IMAGE_FILE := evcc_$(TAG_NAME).image
IMAGE_ROOTFS := evcc_$(TAG_NAME).rootfs
IMAGE_OPTIONS := -hostname evcc -http_port 8080 github.com/gokrazy/serial-busybox github.com/gokrazy/breakglass github.com/evcc-io/evcc

default: build

all: clean install install-ui ui assets lint test-ui lint-ui test build

clean:
	rm -rf dist/

install:
	go install $$(go list -f '{{join .Imports " "}}' tools.go)

install-ui:
	npm ci

ui:
	npm run build

assets:
	go generate ./...

lint:
	golangci-lint run

lint-ui:
	npm run lint

test-ui:
	npm test

test:
	@echo "Running testsuite"
	go test $(BUILD_TAGS) ./...

build:
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	go build -v $(BUILD_TAGS) $(BUILD_ARGS)

release-test:
	goreleaser --snapshot --skip-publish --rm-dist

release:
	goreleaser --rm-dist

docker:
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	docker build --tag $(DOCKER_IMAGE):testing .

publish-testing:
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	seihon publish --dry-run=false --template docker/tmpl.Dockerfile --base-runtime-image alpine:$(ALPINE_VERSION) \
	   --image-name $(DOCKER_IMAGE) -v "testing" --targets=$(TARGETS)

publish-latest:
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	seihon publish --dry-run=false --template docker/tmpl.Dockerfile --base-runtime-image alpine:$(ALPINE_VERSION) \
	   --image-name $(DOCKER_IMAGE) -v "latest" --targets=$(TARGETS)

publish-nightly:
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	seihon publish --dry-run=false --template docker/ci.Dockerfile --base-runtime-image alpine:$(ALPINE_VERSION) \
	   --image-name $(DOCKER_IMAGE) -v "nightly" --targets=$(TARGETS)

publish-release:
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	RELEASE=1 seihon publish --dry-run=false --template docker/ci.Dockerfile --base-runtime-image alpine:$(ALPINE_VERSION) \
	   --image-name $(DOCKER_IMAGE) -v "latest" -v "$(TAG_NAME)" --targets=$(TARGETS)

# gokrazy image
prepare-image:
	go install github.com/gokrazy/tools/cmd/gokr-packer@latest
	mkdir -p flags/github.com/gokrazy/breakglass
	echo "-forward=private-network" > flags/github.com/gokrazy/breakglass/flags.txt
	mkdir -p buildflags/github.com/evcc-io/evcc
	echo "$(BUILD_TAGS),gokrazy" > buildflags/github.com/evcc-io/evcc/buildflags.txt
	echo "$(BUILD_ARGS)" >> buildflags/github.com/evcc-io/evcc/buildflags.txt

image:
	gokr-packer -overwrite=$(IMAGE_FILE) -target_storage_bytes=1258299392 $(IMAGE_OPTIONS)
	loop=$$(sudo losetup --find --show -P $(IMAGE_FILE)); sudo mkfs.ext4 $${loop}p4
	gzip -f $(IMAGE_FILE)

image-rootfs:
	gokr-packer -overwrite_root=$(IMAGE_ROOTFS) $(IMAGE_OPTIONS)
	gzip -f $(IMAGE_ROOTFS)

image-update:
	gokr-packer -update yes $(IMAGE_OPTIONS)

soc:
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	go build $(BUILD_TAGS) $(BUILD_ARGS) github.com/evcc-io/evcc/cmd/soc

stamps:
	docker pull hacksore/hks
	docker run --platform linux/amd64 --rm hacksore/hks hyundai list 014d2225-8495-4735-812d-2616334fd15d | head -n 101 | tail -n 100 > vehicle/bluelink/014d2225-8495-4735-812d-2616334fd15d
	docker run --platform linux/amd64 --rm hacksore/hks kia list 693a33fa-c117-43f2-ae3b-61a02d24f417 | head -n 101 | tail -n 100 > vehicle/bluelink/693a33fa-c117-43f2-ae3b-61a02d24f417
