BIN ?= dist/ccbox
TAG ?= s12chung/ccbox:latest

GOARCH ?= $(shell go env GOARCH)

build:
	go run ./toolsbuild -goarch $(GOARCH) -o dist/ccboxtools
	go build -o $(BIN)
	GOARCH=$(GOARCH) $(BIN) doctor tools

lint:
	hadolint Dockerfile
	shellcheck pkg/harness/clis/claude/config/statusline.sh tests/test_helper.bash tests/*.bats
	find pkg/harness/clis -name '*.json' -exec jq empty {} +
	find pkg/projectcfg/testdata -name '*.yaml' -exec yq '.' {} + > /dev/null

	go run ./toolsbuild -goarch $(GOARCH) -o dist/ccboxtools # needed to build for lint
	golangci-lint run --fix $(TEST)
	cd ccboxtools && golangci-lint run --fix $(TEST)

ci: lint test
test.all: test test.docker

test: lint
	cd ccboxtools && go test -race -count=2 ./... # -race -count=2 for ccboxtools' lock interplay
	go test ./...

# Manual: needs the built image + network egress, so it stays out of CI.
test.docker:
	bats tests/

docker.test: build
	$(BIN) build --tag $(TAG)
	docker run --rm \
	-v ccbox-clis:/opt/ccbox/clis \
	-v $(shell pwd):/home/ccbox/docker.test \
	--workdir /home/ccbox/docker.test \
	-e CLI_PKGINFO="$$( $(BIN) pkginfo )" \
	$(TAG) \
	-lc 'make test.all'
