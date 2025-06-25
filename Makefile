PLUGIN_NAME=kubectl-ball
VERSION=v0.1.0
BUILD_DIR=release
DOCKER_IMAGE=kubectl-ball

PLATFORMS = \
  linux_amd64 \
  linux_arm64 \
  darwin_amd64 \
  darwin_arm64

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

# Docker targets
docker-build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

docker-run:
	docker run -it --rm \
		-v $(HOME)/.kube:/root/.kube:ro \
		-v $(PWD):/workspace \
		-e KUBECONFIG=/root/.kube/config \
		$(DOCKER_IMAGE):latest $(ARGS)

docker-shell:
	docker run -it --rm \
		-v $(HOME)/.kube:/root/.kube:ro \
		-v $(PWD):/workspace \
		-e KUBECONFIG=/root/.kube/config \
		$(DOCKER_IMAGE):latest /bin/bash
