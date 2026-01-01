GO = go
APP_NAME = drunkexec-server
BUILD_DIR = build
ENTRYPOINT = cmd/$(APP_NAME)/main.go
TARGET = $(BUILD_DIR)/$(APP_NAME)

GO_SRC = $(shell find . -type f -name "*.go")
MOD_SRC = $(shell find . -type f -name "*.mod")
ALL_SRC = $(GO_SRC) $(MOD_SRC)

$(shell mkdir -p $(BUILD_DIR))

GIT_COMMIT	  := $(shell git rev-parse --short HEAD || echo 'unknown')
GIT_BRANCH	  := $(shell echo $${WORKFLOW_BRANCH_OR_TAG-$$(git rev-parse --abbrev-ref HEAD || echo 'unknown')})
GIT_BRANCH_NUM  := $(shell git rev-list --count HEAD || echo 'nan')
BUILD_DATE	  := $(shell date '+%d-%m-%Y' || echo 'unknown')
BUILD_TIME	  := $(shell date '+%H:%M:%S' || echo 'unknown')
VERSION		 := $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null || echo 'unknown')

GO_FLAGS := CGO_ENABLED=0

LDFLAGS = \
	-X $(APP_NAME)/internal/models.gitCommit=$(GIT_COMMIT) \
	-X $(APP_NAME)/internal/models.gitBranch=$(GIT_BRANCH) \
	-X $(APP_NAME)/internal/models.gitBranchNum=$(GIT_BRANCH_NUM) \
	-X $(APP_NAME)/internal/models.buildDate=$(BUILD_DATE) \
	-X $(APP_NAME)/internal/models.buildTime=$(BUILD_TIME) \
	-X $(APP_NAME)/internal/models.version=$(VERSION)

all: $(TARGET)

#@GOOS=linux GOARCH=amd64 $(GO) build
$(TARGET): $(ALL_SRC) Makefile
	@echo "\tGO\t$@"
	@$(GO_FLAGS) $(GO) build \
		-ldflags "$(LDFLAGS)" \
		-o $(TARGET) \
		$(ENTRYPOINT)

.PHONY: run
run: $(TARGET)
	@./$(TARGET)

.PHONY: format
format:
	@for CUR in $(GO_SRC); do \
		echo "\tFMT\t$$CUR"; \
		$(GO) fmt $$CUR; \
	done

.PHONY: clean
clean:
	@echo "\tRM\t$(BUILD_DIR)"
	@-rm -rf $(BUILD_DIR)

