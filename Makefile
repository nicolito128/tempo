# Binary output name
BIN ?= tempo

# Package name
PKG := github.com/nicolito128/tempo

# Architecture
ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)

# Git branch
VERSION ?= main

# Output directory
OUTPUT_DIR ?= _output

# Go environment
platform = $(subst -, ,$(ARCH))
GOOS = $(word 1, $(platform))
GOARCH = $(word 2, $(platform))
GOPROXY ?= "https://proxy.golang.org,direct"

.PHONY: all clean tidy run install

all:
	@$(MAKE) build

build: $(OUTPUT_DIR)/bin/$(GOOS)/$(GOARCH)/$(BIN)

$(OUTPUT_DIR)/bin/$(GOOS)/$(GOARCH)/$(BIN): build-dirs
	@echo "building: $@"
		GOOS=$(GOOS) \
		GOARCH=$(GOARCH) \
		VERSION=$(VERSION) \
		PKG=$(PKG) \
		BIN=$(BIN) \
		OUTPUT_DIR=$$(pwd)/$(OUTPUT_DIR)/bin/$(GOOS)/$(GOARCH) \
		./scripts/build.sh

build-dirs:
	@mkdir -p $(OUTPUT_DIR)/bin/$(GOOS)/$(GOARCH)

clean:
	@echo "cleaning output directory"
	@rm -rf $(OUTPUT_DIR)

tidy:
	go mod tidy

run: build
	$$(pwd)/$(OUTPUT_DIR)/bin/$(GOOS)/$(GOARCH)/$(BIN) $(ARGS)

install: build
	@echo "installing: $(BIN) to /usr/local/bin"
	@cp $(OUTPUT_DIR)/bin/$(GOOS)/$(GOARCH)/$(BIN) /usr/local/bin/$(BIN)

