PLUGIN_NAME=kubectl-ball
VERSION=v0.1.0
BUILD_DIR=release

PLATFORMS = \
  linux_amd64 \
  darwin_amd64

all: clean build package

build:
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%_*} GOARCH=$${platform#*_} go build -o $(BUILD_DIR)/$(PLUGIN_NAME)_$$platform main.go; \
	done

package:
	@for platform in $(PLATFORMS); do \
		cd $(BUILD_DIR) && tar -czf $(PLUGIN_NAME)_$$platform.tar.gz $(PLUGIN_NAME)_$$platform && cd ..; \
	done

sha256:
	@for platform in $(PLATFORMS); do \
		shasum -a 256 $(BUILD_DIR)/$(PLUGIN_NAME)_$$platform.tar.gz; \
	done

clean:
	rm -rf $(BUILD_DIR)
