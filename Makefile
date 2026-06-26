GO ?= go
APP ?= seacloud
VERSION ?= $(shell node -p "require('./package.json').version" 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo dev)
PREFIX ?= /usr/local

SEACLOUD_BASE_URL ?= https://cloud.seaart.ai
SEACLOUD_MODELS_URL ?= https://cloud-model-spec.vtrix.ai
SEACLOUD_GENERATION_URL ?= $(SEACLOUD_BASE_URL)
SEACLOUD_SKILLHUB_URL ?= https://skill-hub.vtrix.ai/api/v1
SEACLOUD_FOLKOS_PROXY_URL ?=

LDFLAGS := -s -w \
	-X github.com/SeaCloudAI/seacloud-cli/internal/buildinfo.Version=$(VERSION) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/auth.BaseURL=$(SEACLOUD_BASE_URL) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/account.BaseURL=$(SEACLOUD_BASE_URL) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/sandbox.BaseURL=$(SEACLOUD_BASE_URL) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/models.BaseURL=$(SEACLOUD_MODELS_URL) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/contracts.BaseURL=$(SEACLOUD_MODELS_URL) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/generation.BaseURL=$(SEACLOUD_GENERATION_URL) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/queue.BaseURL=$(SEACLOUD_GENERATION_URL) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/skillhub.BaseURL=$(SEACLOUD_SKILLHUB_URL) \
	-X github.com/SeaCloudAI/seacloud-cli/internal/config.DefaultFolkosProxyBaseURL=$(SEACLOUD_FOLKOS_PROXY_URL)

.PHONY: build install uninstall clean

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(APP) .

install: build
	install -d $(PREFIX)/bin
	install -m755 $(APP) $(PREFIX)/bin/$(APP)
	@echo "installed $(APP) to $(PREFIX)/bin/$(APP)"

uninstall:
	rm -f $(PREFIX)/bin/$(APP)

clean:
	rm -f "$(APP)"
