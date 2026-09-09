BIN ?= dist/ccbox
TAG ?= s12chung/ccbox:latest

build: dist/ccboxtools.tar.gz
	go build -o $(BIN)
	$(BIN) doctor tools

# ccboxtools is a nested module, so go:embed can't take its dir, so we pack a tar instead.
dist/ccboxtools.tar.gz: Makefile $(shell find ccboxtools -type f)
	mkdir -p dist
	# exclude keep macOS bsdtar from adding AppleDouble `._` entries
	COPYFILE_DISABLE=1 tar --no-xattrs --exclude='._*' -czf $@ ccboxtools

# tar needed to build with go:embed, which allows lint
lint: dist/ccboxtools.tar.gz
	hadolint Dockerfile
	shellcheck pkg/harness/clis/claude/config/statusline.sh tests/test_helper.bash tests/*.bats
	find pkg/harness/clis -name '*.json' -exec jq empty {} +
	find pkg/projectcfg/testdata -name '*.yaml' -exec yq '.' {} + > /dev/null
	golangci-lint run --fix $(TEST)
	cd ccboxtools && golangci-lint run --fix $(TEST)

ci: lint test
test.all: test test.docker

test: lint
	go test ./...
	cd ccboxtools && go test ./...

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
