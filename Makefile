GO=CGO_ENABLED=0 godep go
GO_BUILD_OPTS=-a --tags netgo --ldflags '-extldflags "-static"'
SRCDIR=./src
SRC=$(SRCDIR)/...
MAIN=$(SRCDIR)/main.go
TEST_OPTS=-v
DIST=./dist
EXE_NAME=iam-docker
EXE=$(DIST)/$(EXE_NAME)
DOCKER=docker
DOCKER_BUILD_IMAGE_NAME=iam-docker-build
DOCKER_IMAGE_NAME=iam-docker
DOCKER_TAG=$(shell git rev-parse --quiet --short HEAD)
DOCKER_BUILD_IMAGE=$(DOCKER_BUILD_IMAGE_NAME):$(DOCKER_TAG)
DOCKER_BUILD_EXE=/go/src/github.com/swipely/iam-docker/dist/iam-docker
DOCKER_IMAGE=$(DOCKER_IMAGE_NAME):$(DOCKER_TAG)
RELEASE_DOCKERFILE=Dockerfile.build
RELEASE_DOCKERFILE=Dockerfile.release

default: test

build:
	$(GO) build $(SRC)

test:
	$(GO) test $(TEST_OPTS) $(SRC)

exe: $(EXE)

clean:
	rm -rf $(DIST)

docker: clean
	$(DOCKER) build -t $(DOCKER_BUILD_IMAGE) -f $(BUILD_DOCKERFILE) .
	$(eval CONTAINER := $(shell $(DOCKER) create $(DOCKER_BUILD_IMAGE) /bin/sleep 100))
	mkdir -p $(DIST)
	$(DOCKER) cp $(CONTAINER):$(DOCKER_BUILD_EXE) $(EXE)
	$(DOCKER) rm -f $(CONTAINER)
	$(DOCKER) build -t $(DOCKER_IMAGE_NAME) -f $(RELEASE_DOCKERFILE) .

$(EXE): clean $(DIST)
	$(GO) build $(GO_BUILD_OPTS) -o $(EXE) $(MAIN)

$(DIST):
	mkdir -p $(DIST)

.PHONY: build clean default docker exe test
