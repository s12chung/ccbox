BIN ?= dist/ccbox
TAG ?= s12chung/ccbox:latest

build:
	go build -o $(BIN)

lint:
	hadolint Dockerfile
	shellcheck docker/image/entrypoint.sh docker/seed/claude-config/statusline.sh tests/test_helper.bash tests/*.bats
	jq empty docker/seed/claude-config/settings.json
	gofmt -l . | (! grep .)
	go vet ./...

ci: test
test.all: test test.docker

test: lint
	go test ./...

# Manual: needs the built image + network egress, so it stays out of CI.
test.docker:
	bats tests/

docker.test: build
	$(BIN) build --tag $(TAG)
	docker run --rm \
	-v $(shell pwd):/home/ccbox/workspace \
	--entrypoint bash $(TAG) \
	-lc 'make test.all'
