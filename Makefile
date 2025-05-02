PROJECT_NAME := kuma-disgo
BINARY_NAME := $(PROJECT_NAME)
BUILD_DIR := build
LDFLAGS := -ldflags "-s -w"
UPX_OPTIONS := --best -q
DEB_DIR := $(BUILD_DIR)/$(PROJECT_NAME)-deb
VERSION := 0.1.0
ARCH := $(shell dpkg --print-architecture)
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
PACKAGE_BASENAME := $(BINARY_NAME)_v$(VERSION)_$(OS)_$(ARCH)

# Flags
COMPRESS ?= 0
FORCE_BUILD ?= 0

.PHONY: all build compress clean install user-install help deb tarball deps

all: build

deps:
	@echo "Checking dependencies..."
	@which go >/dev/null 2>&1 || (echo "Error: Go is not installed. Please install Go from https://golang.org/dl/"; exit 1)
	@go version
	@echo "Dependencies check complete!"

build: deps
	@if [ "$(FORCE_BUILD)" = "1" ] || [ ! -f "$(BINARY_NAME)" ]; then \
		echo "Building..."; \
		cd src && go mod tidy && go build $(LDFLAGS) -o ../$(BINARY_NAME); \
		echo "Build complete!"; \
	else \
		echo "Binary already exists, skipping build..."; \
	fi

compress: build
	@echo "Checking for UPX..."
	@which upx >/dev/null 2>&1 && ( \
		echo "Compressing with UPX..."; \
		upx $(UPX_OPTIONS) $(BINARY_NAME); \
		echo "Compression complete!"; \
	) || printf "Skipping compression (UPX not found)\n\
	To enable compression, install UPX from: https://upx.github.io/\n"

clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)/$(PACKAGE_BASENAME)*
	rm -rf $(DEB_DIR)
	@echo "Clean complete!"

install: build
	@echo "Installing to /usr/local/bin..."
	install -D $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installation complete!"

user-install: build
	@echo "Installing to user's local bin directory..."
	mkdir -p ~/.local/bin
	install -D $(BINARY_NAME) ~/.local/bin/$(BINARY_NAME)
	@echo "Installation to user's local bin directory complete!"
	@echo "Don't forget to add \$$HOME/.local/bin to \$$PATH:"
	@echo "  export PATH=\"\$$HOME/.local/bin:\$$PATH\""

deb: build
	@if [ "$(COMPRESS)" = "1" ]; then \
		$(MAKE) compress; \
	fi
	@echo "Packaging .deb file..."
	rm -rf $(DEB_DIR)
	mkdir -p $(DEB_DIR)/DEBIAN
	mkdir -p $(DEB_DIR)/usr/local/bin
	install -m 0755 $(BINARY_NAME) $(DEB_DIR)/usr/local/bin/$(BINARY_NAME)
	@echo "Package: $(PROJECT_NAME)" > $(DEB_DIR)/DEBIAN/control
	@echo "Version: $(VERSION)" >> $(DEB_DIR)/DEBIAN/control
	@echo "Section: utils" >> $(DEB_DIR)/DEBIAN/control
	@echo "Priority: optional" >> $(DEB_DIR)/DEBIAN/control
	@echo "Architecture: $(ARCH)" >> $(DEB_DIR)/DEBIAN/control
	@echo "Maintainer: Your Name <your.email@example.com>" >> $(DEB_DIR)/DEBIAN/control
	@echo "Description: Discord bot for displaying Uptime Kuma service statuses" >> $(DEB_DIR)/DEBIAN/control
	chmod -R 0755 $(DEB_DIR)
	dpkg-deb --build $(DEB_DIR) $(BUILD_DIR)/$(PACKAGE_BASENAME).deb
	@echo "Debian package created: $(BUILD_DIR)/$(PACKAGE_BASENAME).deb"

tarball: build
	@if [ "$(COMPRESS)" = "1" ]; then \
		$(MAKE) compress; \
	fi
	@echo "Creating tar.gz archive..."
	rm -f $(BUILD_DIR)/$(PACKAGE_BASENAME).tar.gz
	mkdir -p $(BUILD_DIR)/tarball
	cp $(BINARY_NAME) $(BUILD_DIR)/tarball/
	cp README.md $(BUILD_DIR)/tarball/
	cp config.yaml.example $(BUILD_DIR)/tarball/config.yaml
	tar -czf $(BUILD_DIR)/$(PACKAGE_BASENAME).tar.gz -C $(BUILD_DIR)/tarball $(BINARY_NAME) README.md config.yaml
	rm -rf $(BUILD_DIR)/tarball
	@echo "Created: $(BUILD_DIR)/$(PACKAGE_BASENAME).tar.gz"

help:
	@echo "Usage: make [target] [COMPRESS=1] [FORCE_BUILD=1]"
	@echo "Targets:"
	@echo "  all            Builds the binary"
	@echo "  build          Builds the binary (requires Go)"
	@echo "  compress       Compresses the binary with UPX (optional)"
	@echo "  clean          Cleans the build directory"
	@echo "  install        Installs the binary to /usr/local/bin"
	@echo "  user-install   Installs the binary to ~/.local/bin"
	@echo "  deb            Builds a deb package"
	@echo "  tarball        Creates a tar.gz archive"
	@echo "  help           Shows this help message"
	@echo ""
	@echo "Options:"
	@echo "  COMPRESS=1     Enable compression for deb/tarball targets"
	@echo "  FORCE_BUILD=1  Force rebuild even if binary exists" 