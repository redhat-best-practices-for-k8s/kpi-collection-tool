# Detect the operating system
UNAME_S := $(shell uname -s)
# Binary name
BINARY_NAME=kpi-collector
# Module path for go install
MODULE_PATH=github.com/redhat-best-practices-for-k8s/kpi-collection-tool

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/kpi-collector

build-debug:
	CGO_ENABLED=0 go build -gcflags="all=-N -l" -o $(BINARY_NAME) ./cmd/kpi-collector

build-dynamic-linking:
	CGO_ENABLED=1 go build -ldflags="-s -w -extldflags '-z relro -z now'" -o $(BINARY_NAME) ./cmd/kpi-collector

build-dynamic-linking-debug:
	CGO_ENABLED=1 go build -gcflags="all=-N -l" -o $(BINARY_NAME) ./cmd/kpi-collector

# Mac installation via Homebrew
install-golangci-lint-mac:
	brew install golangci-lint

# Linux/CI installation via go install
install-golangci-lint-linux:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

# Platform-agnostic golangci-lint installation
install-golangci-lint:
ifeq ($(UNAME_S),Darwin)
	$(MAKE) install-golangci-lint-mac
else
	$(MAKE) install-golangci-lint-linux
endif

# Lint depends only on golangci-lint installation
lint: install-golangci-lint
	golangci-lint run --timeout 10m0s

# Run all tests
test:
	go test ./... -v

# Install kpi-collector to user's Go bin directory
install:
	go install ./cmd/kpi-collector
	echo "✓ Installed to $(HOME)/go/bin/$(BINARY_NAME)"

# Uninstall kpi-collector
uninstall:
	rm -f $(HOME)/go/bin/$(BINARY_NAME)
	echo "✓ Uninstalled"

# Install kpi-collector from upstream (no local source needed)
install-kpi-collector:
	go install $(MODULE_PATH)/cmd/kpi-collector@latest
