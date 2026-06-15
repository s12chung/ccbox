REPO ?= s12chung/ccbox
VERSION ?= latest
TAG ?= $(REPO):$(VERSION)

CACHE_DIR ?= $(HOME)/.ccbox

run: build
	docker network connect bridge ccbox-egress || true
	mkdir -p $(CACHE_DIR)/claude-config
	docker run -it --rm \
	--network ccbox-wall \
	--cap-drop=ALL \
	--security-opt=no-new-privileges \
	-e http_proxy=http://ccbox-egress:8888 \
	-e https_proxy=http://ccbox-egress:8888 \
	-v $(CACHE_DIR)/claude-config:/home/ccbox/claude-config \
	-v $(shell pwd):/home/ccbox/workspace \
	-e CLAUDE_CODE_OAUTH_TOKEN=$(CLAUDE_CODE_OAUTH_TOKEN) \
	-e GH_TOKEN=$(GH_TOKEN) \
	$(TAG)

build:
	docker build -t $(TAG) .

lint:
	hadolint Dockerfile
	shellcheck docker/image/entrypoint.sh tests/test_helper.bash tests/*.bats
	ruby tests/regexp_file_test.rb

test.local: lint
	# Fill in later

# Manual: needs the built image + network egress, so it stays out of CI.
test.docker: build
	docker run --rm \
	-v $(shell pwd):/home/ccbox/workspace \
	--entrypoint bash $(TAG) \
	-lc 'bats tests/'

test: test.local test.docker

# ci run on the host with no Docker
ci: lint

proxy-clean:
	docker network rm ccbox-wall

proxy:
	mkdir -p $(CACHE_DIR)
	cp -rf docker/tinyproxy $(CACHE_DIR) || true
	docker network create --internal ccbox-wall || true
	docker run --rm --name ccbox-egress --network ccbox-wall \
      -v $(CACHE_DIR)/tinyproxy:/etc/tinyproxy:ro \
      kalaksi/tinyproxy:latest