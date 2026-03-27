IMAGE?=localhost/vllm-ppc64le
TAG?=dev
VLLM_VERSION?=v0.11.1
CONTAINER_BUILDER?=podman

build:
	$(CONTAINER_BUILDER) build \
		--build-arg VLLM_VERSION=$(VLLM_VERSION) \
		-t $(IMAGE):$(TAG) .
.PHONY: build

# Verify the image works
test:
	$(CONTAINER_BUILDER) run --rm $(IMAGE):$(TAG) \
		"python -c 'import vllm; print(\"vLLM version:\", vllm.__version__)'"
.PHONY: test

image-tag:
	@echo $(TAG)
.PHONY: image-tag
