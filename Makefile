# in the container, dist/ is a throwaway tmpfs: build to the persistent /tmp there
BIN ?= $(if $(DEVCONTAINER),/tmp/ccbox,dist/ccbox)
GOARCH ?= $(shell go env GOARCH)

build:
	go run ./tools/toolsbuild -goarch $(GOARCH) -o dist/ccboxtools
	go build -o $(BIN)
	GOARCH=$(GOARCH) $(BIN) doctor tools

lint: lint.terms
	hadolint Dockerfile
	shellcheck docker/desktop.sh docker/web-browser pkg/models/cli/clitmpl/clis/claude/config/statusline.sh tests/test_helper.bash tests/*.bats
	find pkg/models/cli/clitmpl/clis -name '*.json' -exec jq empty {} +
	find pkg/projectcfg/testdata -name '*.yaml' -exec yq '.' {} + > /dev/null

	go run ./tools/toolsbuild -goarch $(GOARCH) -o dist/ccboxtools # needed to build for lint
	golangci-lint run --fix $(TEST)
	cd tools/ccboxtools && golangci-lint run --fix $(TEST)

lint.terms:
	@out=$$(grep -rnE '(_|\b)[Ww][Aa][Ll][Ll]([Ee][Dd]|[Ss])?(_|\b)|[a-z]Wall' \
		--include='*.go' --include='*.md' --include='*.tmpl' --include='*.sh' --include='*.bats' \
		--include='*.yaml' --include='*.yml' --include='*.json' --include='*.toml' . \
		| grep -vE '[Ee]gress[ -]?[Ww]all|wall/walled|^\./(\.git|dist|\.idea)/'); \
	if [ -n "$$out" ]; then echo "$$out"; echo 'standalone "wall/walled" is banned — use "proxy" or "egress wall"' >&2; exit 1; fi

ci: lint test
test.all: test test.docker

test: lint
	cd tools/ccboxtools && go test -race -count=2 ./pkg/util/flock ./pkg/install # -race -count=2 for lock interplay
	cd tools/ccboxtools && go test ./...
	go test ./...

# Manual: needs the built image + network egress, so it stays out of CI.
test.docker:
	bats tests/

docker.test: build
	TERM=xterm-256color $(BIN) run make test.all
