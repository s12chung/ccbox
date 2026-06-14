REPO ?= s12chung/ccbox
VERSION ?= latest
TAG ?= $(REPO):$(VERSION)

CACHE_DIR ?= $(HOME)/.ccbox

run: build
	docker network connect bridge ccbox-egress || true
	docker run -it --rm \
	--network ccbox-wall \
	-e http_proxy=http://ccbox-egress:8888 \
	-e https_proxy=http://ccbox-egress:8888 \
	-v $(CACHE_DIR)/claude-config:/root/claude-config \
	-v $(shell pwd):/root/workspace \
	-e CLAUDE_CODE_OAUTH_TOKEN=$(CLAUDE_CODE_OAUTH_TOKEN) \
	$(TAG)

build:
	docker build -t $(TAG) .

proxy-clean:
	docker network rm ccbox-wall

proxy:
	mkdir -p $(CACHE_DIR)
	cp -r tinyproxy $(CACHE_DIR)/tinyproxy || true
	docker network create --internal ccbox-wall || true
	docker run --rm --name ccbox-egress --network ccbox-wall \
      -v $(CACHE_DIR)/tinyproxy:/etc/tinyproxy:ro \
      kalaksi/tinyproxy:latest