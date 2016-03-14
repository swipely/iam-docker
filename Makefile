GO=CGO_ENABLED=0 godep go
GO_BUILD_OPTS=-a --tags netgo --ldflags '-extldflags "-static"'
SRCDIR=./src
SRC=$(SRCDIR)/...
MAIN=$(SRCDIR)/main.go
TEST_OPTS=-v
DIST=./dist
EXE_NAME=iam-docker
EXE=$(DIST)/$(EXE_NAME)
CACERT=$(DIST)/ca-certificates.crt
CACERT_SRC=https://curl.haxx.se/ca/cacert.pem
DOCKER=docker
DOCKER_BUILD_IMAGE_NAME=swipely/iam-docker-build
DOCKER_RELEASE_IMAGE_NAME=swipely/iam-docker
DOCKER_TAG=$(shell git rev-parse --quiet --short HEAD)
DOCKER_BUILD_IMAGE=$(DOCKER_BUILD_IMAGE_NAME):$(DOCKER_TAG)
DOCKER_RELEASE_IMAGE=$(DOCKER_RELEASE_IMAGE_NAME):$(DOCKER_TAG)
DOCKER_RELEASE_IMAGE_LATEST=$(DOCKER_RELEASE_IMAGE_NAME):latest
DOCKER_BUILD_EXE=/go/src/github.com/swipely/iam-docker/dist/iam-docker
BUILD_DOCKERFILE=Dockerfile.build
RELEASE_DOCKERFILE=Dockerfile.release

default: test

clean:
	rm -rf $(DIST)

build:
	$(GO) build $(SRC)

test:
	$(GO) test $(TEST_OPTS) $(SRC)

exe: $(EXE)

get-deps:
	go get -u github.com/tools/godep

test-in-docker: docker-build
	$(DOCKER) run $(DOCKER_BUILD_IMAGE) make test

release: docker
	docker push $(DOCKER_RELEASE_IMAGE)
	docker push $(DOCKER_RELEASE_IMAGE_LATEST)

docker: docker-build $(CACERT)
	$(eval CONTAINER := $(shell $(DOCKER) create $(DOCKER_BUILD_IMAGE) make exe))
	$(DOCKER) start $(CONTAINER)
	$(DOCKER) logs -f $(CONTAINER)
	mkdir -p $(DIST)
	$(DOCKER) cp $(CONTAINER):$(DOCKER_BUILD_EXE) $(EXE)
	$(DOCKER) rm -f $(CONTAINER)
	$(DOCKER) build -t $(DOCKER_RELEASE_IMAGE) -f $(RELEASE_DOCKERFILE) .
	$(DOCKER) tag $(DOCKER_RELEASE_IMAGE) $(DOCKER_RELEASE_IMAGE_LATEST)

docker-build:
	$(DOCKER) build -t $(DOCKER_BUILD_IMAGE) -f $(BUILD_DOCKERFILE) .

$(CACERT): $(DIST)
	curl -s $(CACERT_SRC) > $(CACERT)

$(EXE): $(DIST)
	rm -f $(EXE)
	$(GO) build $(GO_BUILD_OPTS) -o $(EXE) $(MAIN)

$(DIST):
	mkdir -p $(DIST)

.PHONY: build clean default docker docker-build exe get-deps release test test-in-docker
