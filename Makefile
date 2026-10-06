BIN ?= dist/ccbox
TAG ?= s12chung/ccbox:latest

GOARCH ?= $(shell go env GOARCH)

build:
	go run ./toolsbuild -goarch $(GOARCH) -o dist/ccboxtools
	go build -o $(BIN)
	GOARCH=$(GOARCH) $(BIN) doctor tools

lint: lint.terms
	hadolint Dockerfile
	shellcheck docker/desktop.sh docker/web-browser pkg/cli/clitmpl/clis/claude/config/statusline.sh tests/test_helper.bash tests/*.bats
	find pkg/cli/clitmpl/clis -name '*.json' -exec jq empty {} +
	find pkg/projectcfg/testdata -name '*.yaml' -exec yq '.' {} + > /dev/null

	go run ./toolsbuild -goarch $(GOARCH) -o dist/ccboxtools # needed to build for lint
	golangci-lint run --fix $(TEST)
	cd ccboxtools && golangci-lint run --fix $(TEST)

lint.terms:
	@out=$$(grep -rnE '(_|\b)[Ww][Aa][Ll][Ll]([Ee][Dd]|[Ss])?(_|\b)|[a-z]Wall' \
		--include='*.go' --include='*.md' --include='*.tmpl' --include='*.sh' --include='*.bats' \
		--include='*.yaml' --include='*.yml' --include='*.json' --include='*.toml' . \
		| grep -vE '[Ee]gress[ -]?[Ww]all|wall/walled|^\./(\.git|dist|\.idea)/'); \
	if [ -n "$$out" ]; then echo "$$out"; echo 'standalone "wall/walled" is banned — use "proxy" or "egress wall"' >&2; exit 1; fi

ci: lint test
test.all: test test.docker

test: lint
	cd ccboxtools && go test -race -count=2 ./pkg/util/flock ./pkg/install # -race -count=2 for lock interplay
	cd ccboxtools && go test ./...
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
