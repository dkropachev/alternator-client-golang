define dl_tgz
	@if ! $(1) 2>/dev/null 1>&2; then \
		[ -d "$(GOBIN)" ] || mkdir "$(GOBIN)"; \
		if [ ! -f "$(GOBIN)/$(1)" ]; then \
			echo "Downloading $(GOBIN)/$(1)"; \
			curl --progress-bar -L $(2) | tar zxf - --wildcards --strip 1 -C $(GOBIN) '*/$(1)'; \
			chmod +x "$(GOBIN)/$(1)"; \
		fi; \
	fi
endef

define dl_bin
	@if ! $(1) 2>/dev/null 1>&2; then \
		[ -d "$(GOBIN)" ] || mkdir "$(GOBIN)"; \
		if [ ! -f "$(GOBIN)/$(1)" ]; then \
			echo "Downloading $(GOBIN)/$(1)"; \
			curl --progress-bar -L $(2) --output "$(GOBIN)/$(1)"; \
			chmod +x "$(GOBIN)/$(1)"; \
		fi; \
	fi
endef

MAKEFILE_PATH := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
GOOS := $(shell uname | tr '[:upper:]' '[:lower:]')
GOARCH := $(shell go env GOARCH)
DOCKER_COMPOSE_VERSION := 2.34.0
GOLANGCI_VERSION := v2.1.6

ifeq ($(GOARCH),arm64)
	DOCKER_COMPOSE_DOWNLOAD_URL := "https://github.com/docker/compose/releases/download/v$(DOCKER_COMPOSE_VERSION)/docker-compose-$(GOOS)-aarch64"
	GOLANGCI_DOWNLOAD_URL := "https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_VERSION)/golangci-lint-$(GOLANGCI_VERSION)-$(GOOS)-arm64.tar.gz"
else ifeq ($(GOARCH),amd64)
	DOCKER_COMPOSE_DOWNLOAD_URL := "https://github.com/docker/compose/releases/download/v$(DOCKER_COMPOSE_VERSION)/docker-compose-$(GOOS)-x86_64"
	GOLANGCI_DOWNLOAD_URL := "https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_VERSION)/golangci-lint-$(GOLANGCI_VERSION)-$(GOOS)-amd64.tar.gz"
else
	@printf 'Unknown architecture "%s"\n', "$(GOARCH)"
	@exit 69
endif


ifndef GOBIN
export GOBIN := $(MAKEFILE_PATH)/bin
endif

export PATH := $(GOBIN):$(PATH)

COMPOSE := docker-compose -f $(MAKEFILE_PATH)/test/docker-compose.yml

.PHONY: clean
clean:
	@echo "Cleaning v1"
	@go clean -r ./...
	@echo "Cleaning v2"
	@cd v2; go clean -r ./...

.PHONY: build
build:
	@echo "Building v1"
	@go build ./...
	@echo "Building v2"
	@go build ./v2/...

.PHONY: clean-caches
clean-caches:
	@go clean -r -cache -testcache -modcache ./...

.PHONY: check
check: check-golangci

.PHONY: fix
fix: fix-golangci

.PHONY: check-golangci
check-golangci: .prepare-golangci
	@echo "======== Lint code for shared"
	@golangci-lint run ./shared/...
	@echo "======== Lint code for v1"
	@golangci-lint run ./...
	@echo "======== Lint code for v2"
	@golangci-lint run ./v2/...

.PHONY: fix-golangci
fix-golangci: .prepare-golangci
	@echo "======== Fix code for shared"
	@golangci-lint run --fix ./shared/...
	@echo "======== Fix code for v1"
	@golangci-lint run --fix ./...
	@echo "======== Fix code for v2"
	@golangci-lint run  --fix ./v2/...

.PHONY: test
test: build check test-unit test-integration

.PHONY: test-unit
test-unit:
	@echo "======== Running unit tests for shared"
	@go test -v -cover -race ./shared/...
	@echo "======== Running unit tests for v1"
	@go test -v -cover -race ./...
	@echo "======== Running unit tests for v2"
	@go test -v -cover -race ./v2/...

.PHONY: test-integration
test-integration: scylla-start
	@echo "======== Running unit tests for v1"
	@go test -v -cover -race -tags integration ./...
	@echo "======== Running unit tests for v2"
	@go test -v -cover -race -tags integration ./v2/...

.PHONY: .prepare-cert
.prepare-cert:
	@[ -f "test/scylla/db.key" ] || (echo "Prepare certificate" && cd test/scylla/ && openssl req -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com" -x509 -newkey rsa:4096 -keyout db.key -out db.crt -days 3650 -nodes && chmod 644 db.key)

.PHONY: scylla-start
scylla-start: .prepare-cert $(GOBIN)/docker-compose
	@sudo sysctl -w fs.aio-max-nr=10485760
	$(COMPOSE) up -d

.PHONY: scylla-stop
scylla-stop: $(GOBIN)/docker-compose
	$(COMPOSE) down

.PHONY: scylla-kill
scylla-kill: $(GOBIN)/docker-compose
	$(COMPOSE) kill

.PHONY: scylla-rm
scylla-rm: $(GOBIN)/docker-compose
	$(COMPOSE) rm -f

.prepare-golangci:
	@if ! golangci-lint --version 2>/dev/null | grep ${GOLANGCI_VERSION} >/dev/null; then \
  		echo "Installing golangci-ling ${GOLANGCI_VERSION}"; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_VERSION}; \
  	fi

$(GOBIN)/docker-compose: Makefile
	$(call dl_bin,docker-compose,$(DOCKER_COMPOSE_DOWNLOAD_URL))
