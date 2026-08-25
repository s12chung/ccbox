BIN ?= dist/ccbox
TAG ?= s12chung/ccbox:latest

build:
	go build -o $(BIN)

lint:
	hadolint Dockerfile
	shellcheck docker/image/entrypoint.sh pkg/harness/clis/claude/config/statusline.sh tests/test_helper.bash tests/*.bats
	find pkg/harness/clis -name '*.json' -exec jq empty {} +
	golangci-lint run --fix $(TEST)

ci: lint test
test.all: test test.docker

test: lint
	go test ./...

# Manual: needs the built image + network egress, so it stays out of CI.
test.docker:
	bats tests/

docker.test: build
	$(BIN) build --tag $(TAG)
	docker run --rm \
	-v $(shell pwd):/home/ccbox/docker.test \
	--workdir /home/ccbox/docker.test \
	--entrypoint bash $(TAG) \
	-lc 'make test.all'
