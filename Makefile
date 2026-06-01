CARGO ?= cargo
APP ?= seacloud
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PREFIX ?= /usr/local

SEACLOUD_BASE_URL ?= https://cloud.seaart.ai
SEACLOUD_MODELS_URL ?= https://cloud-model-spec.vtrix.ai
SEACLOUD_GENERATION_URL ?= $(SEACLOUD_BASE_URL)
SEACLOUD_SKILLHUB_URL ?= https://skill-hub.vtrix.ai/api/v1
SEACLOUD_FOLKOS_PROXY_URL ?=

.PHONY: build install uninstall clean

build:
	SEACLOUD_BUILD_VERSION="$(VERSION)" \
	SEACLOUD_BASE_URL="$(SEACLOUD_BASE_URL)" \
	SEACLOUD_MODELS_URL="$(SEACLOUD_MODELS_URL)" \
	SEACLOUD_GENERATION_URL="$(SEACLOUD_GENERATION_URL)" \
	SEACLOUD_SKILLHUB_URL="$(SEACLOUD_SKILLHUB_URL)" \
	SEACLOUD_FOLKOS_PROXY_URL="$(SEACLOUD_FOLKOS_PROXY_URL)" \
	$(CARGO) build --release
	cp target/release/$(APP) $(APP)

install: build
	install -d $(PREFIX)/bin
	install -m755 $(APP) $(PREFIX)/bin/$(APP)
	@echo "installed $(APP) to $(PREFIX)/bin/$(APP)"

uninstall:
	rm -f $(PREFIX)/bin/$(APP)

clean:
	rm -f "$(APP)"
	rm -rf target
