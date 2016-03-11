GO=godep go
SRCDIR=./src
SRC=$(SRCDIR)/...
TEST_OPTS=-v

build:
	$(GO) build $(SRC)

test:
	$(GO) test $(TEST_OPTS) $(SRC)


.PHONY: get-deps build test
