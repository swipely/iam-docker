GO=godep go
SRCDIR=./src
SRC=$(SRCDIR)/...
MAIN=$(SRCDIR)/main.go
TEST_OPTS=-v
DIST=./dist
EXE_NAME=iam-docker
EXE=$(DIST)/$(EXE_NAME)
DOCKER=docker
DOCKER_IMAGE=iam-docker
DOCKER_TAG=$(shell git rev-parse --quiet --short HEAD)

default: test

build:
	$(GO) build $(SRC)

test:
	$(GO) test $(TEST_OPTS) $(SRC)

exe: $(EXE)

clean:
	rm -rf $(DIST)

docker:
	$(DOCKER) build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

$(EXE): $(DIST) clean
	$(GO) build -o $(EXE) $(MAIN)

$(DIST):
	mkdir -p $(DIST)

.PHONY: default build test clean exe
