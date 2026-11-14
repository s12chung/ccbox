# in the container, dist/ is a throwaway tmpfs: build to the persistent /tmp there
BIN ?= $(if $(DEVCONTAINER),/tmp/ccbox,dist/ccbox)
GOARCH ?= $(shell go env GOARCH)

build:
	go run ./tools/toolsbuild -goarch $(GOARCH) -o dist/ccboxtools
	go build -o $(BIN)
	GOARCH=$(GOARCH) $(BIN) doctor tools

generate:
	go generate ./pkg/cfg
	cd tools/ccboxtools && go generate ./pkg/util/seccomputil

generate.check: generate
	git diff --exit-code -- pkg/cfg/zz_generated.toinnermodel.cfg.go tools/ccboxtools/pkg/util/seccomputil/seccomp_default.json

lint: generate lint.terms
	hadolint pkg/dockerfile/testdata/TestRender.Dockerfile
	shellcheck docker/desktop.sh docker/web-browser pkg/models/cli/clitmpl/clis/claude/config/statusline.sh tests/test_helper.bash tests/*.bats
	find pkg/models/cli/clitmpl/clis -name '*.json' -exec jq empty {} +
	find pkg/cfg/testdata -name '*.yaml' -exec yq '.' {} + > /dev/null

	go run ./tools/toolsbuild -goarch $(GOARCH) -o dist/ccboxtools # needed to build for lint
	golangci-lint run --fix $(TEST)
	cd tools/ccboxtools && golangci-lint run --fix $(TEST)

lint.terms:
	go run ./tools/termlint

ci: generate.check lint test
test.all: test test.docker

test: lint
	cd tools/ccboxtools && go test -race -count=2 ./pkg/util/flock ./pkg/install # -race -count=2 for lock interplay
	cd tools/ccboxtools && go test ./...
	go test ./...

# Manual: needs the built image + network egress, so it stays out of CI.
test.docker:
	bats tests/

docker.test: build
	bats tests/host-docker.bats
	TERM=xterm-256color $(BIN) exec make test.all
