# RictusD V2 Makefile
# Builds both rictusd (daemon) and rictusctl (control agent)
# Output: bin/rictusd and bin/rictusctl

SHELL := /bin/bash

PROJECT_ROOT := $(CURDIR)
BIN_DIR := $(PROJECT_ROOT)/bin
CMD_DIR := $(PROJECT_ROOT)/cmd

GO := go
GOFLAGS := -trimpath
LDFLAGS := -s -w

RDICT := $(BIN_DIR)/rictusd
RCTL  := $(BIN_DIR)/rictusctl

.PHONY: all clean dirs

all: dirs $(RDICT) $(RCTL)

dirs:
	@mkdir -p $(BIN_DIR)

$(RDICT): $(CMD_DIR)/rictusd/main.go
	@echo "Building rictusd..."
	@$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(RDICT) $(CMD_DIR)/rictusd

$(RCTL): $(CMD_DIR)/rictusctl/main.go
	@echo "Building rictusctl..."
	@$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(RCTL) $(CMD_DIR)/rictusctl

clean:
	@echo "Cleaning binaries..."
	@rm -f $(RDICT)
	@rm -f $(RCTL)
	@echo "Clean complete (bin directory preserved)."
