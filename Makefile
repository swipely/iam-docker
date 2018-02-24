GO=CGO_ENABLED=0 godep go
GO_BUILD_OPTS=-a --tags netgo --ldflags '-extldflags "-static"'
SRCDIR=./src
SRC=$(SRCDIR)/...
SRCS=$(SRCDIR)/**/*.go
MAIN=$(SRCDIR)/main.go
TEST_OPTS=-v
VERSION_FILE=VERSION
VERSION=$(shell cat $(VERSION_FILE))
DOCKER=docker
DOCKER_IMAGE_NAME=swipely/iam-docker
DOCKER_TAG=$(VERSION)
DOCKER_BUILD_IMAGE=$(DOCKER_BUILD_IMAGE_NAME):$(DOCKER_TAG)
DOCKER_IMAGE=$(DOCKER_IMAGE_NAME):$(DOCKER_TAG)
DOCKER_IMAGE_LATEST=$(DOCKER_RELEASE_IMAGE_NAME):latest

default: test

build:
	$(GO) build $(SRC)

test:
	$(GO) test $(TEST_OPTS) $(SRC)

get-deps:
	go get -u github.com/tools/godep

release: docker
	git tag $(VERSION)
	git push origin --tags
	docker push $(DOCKER_IMAGE)
	docker push $(DOCKER_IMAGE_LATEST)

docker: $(SRCS) Dockerfile
	$(DOCKER) build -t $(DOCKER_IMAGE) .

.PHONY: build default docker get-deps release test
