GO=godep go
SRCDIR=./src
SRC=$(SRCDIR)/...
MAIN=$(SRCDIR)/main.go
TEST_OPTS=-v
DIST=./dist
EXENAME=iam-docker
EXE=$(DIST)/$(EXENAME)

build:
	$(GO) build $(SRC)

test:
	$(GO) test $(TEST_OPTS) $(SRC)

exe: $(EXE)

$(EXE): $(DIST)
	$(GO) build -o $(EXE) $(MAIN)

$(DIST):
	mkdir -p $(DIST)


.PHONY: build test exe
