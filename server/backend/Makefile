GO ?= go
CGO_ENABLED ?= 1
SRC_DIR := src
BIN_DIR := build
APP_NAME ?= drnkexec-server
CMD_PACKAGE := ./cmd/drnkexec
TARGET := $(BIN_DIR)/$(APP_NAME)
MODULE_PATH := github.com/drunkbatya/drnkexec

GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_BRANCH := $(shell echo $${WORKFLOW_BRANCH_OR_TAG-$$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)})
GIT_BRANCH_NUM := $(shell git rev-list --count HEAD 2>/dev/null || echo nan)
BUILD_DATE := $(shell date '+%d-%m-%Y' 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date '+%H:%M:%S' 2>/dev/null || echo unknown)
BUILD_VERSION := $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null || echo unknown)

LDFLAGS := \
	-X $(MODULE_PATH)/internal/version.gitCommit=$(GIT_COMMIT) \
	-X $(MODULE_PATH)/internal/version.gitBranch=$(GIT_BRANCH) \
	-X $(MODULE_PATH)/internal/version.gitBranchNum=$(GIT_BRANCH_NUM) \
	-X $(MODULE_PATH)/internal/version.buildDate=$(BUILD_DATE) \
	-X $(MODULE_PATH)/internal/version.buildTime=$(BUILD_TIME) \
	-X $(MODULE_PATH)/internal/version.buildVersion=$(BUILD_VERSION)

DOCS := $(SRC_DIR)/docs/docs.go $(SRC_DIR)/docs/swagger.json $(SRC_DIR)/docs/swagger.yaml
NON_DOC_GO := $(shell cd $(SRC_DIR) && find . -path './docs' -prune -o -name '*.go' -print)
NON_DOC_GO := $(addprefix $(SRC_DIR)/,$(filter-out ./docs/docs.go,$(NON_DOC_GO)))
MOD_FILES := $(SRC_DIR)/go.mod $(SRC_DIR)/go.sum

.PHONY: all build run clean test

all: build

build: $(TARGET)

$(TARGET): $(DOCS) $(NON_DOC_GO) $(MOD_FILES)
	@mkdir -p $(BIN_DIR)
	@echo "\tGO\t$(TARGET)"
	@cd $(SRC_DIR) && CGO_ENABLED=$(CGO_ENABLED) $(GO) build \
		-ldflags "$(LDFLAGS)" \
		-o ../$(TARGET) \
		$(CMD_PACKAGE)

run: build
	@echo "\tRUN\t$(TARGET)"
	@./$(TARGET)

test: $(DOCS)
	@echo "\tTEST\t./..."
	@cd $(SRC_DIR) && $(GO) test ./...

generate: $(DOCS)

$(DOCS): $(NON_DOC_GO) $(MOD_FILES)
	@echo "\tGEN\tswagger"
	@cd $(SRC_DIR) && $(GO) generate ./...

clean:
	@echo "\tRM\t$(BIN_DIR)"
	@rm -rf $(BIN_DIR)
