# Makefile for building dns

# Project binaries
COMMANDS=ludns resolvcache resolvcheck resolvcollect
BINARIES=$(addprefix bin/,$(COMMANDS))

# Used to populate version in binaries
VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always)
REVISION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .m; fi)
DATEBUILD=$(shell date +%FT%T%z)

# Compilation opts
GOPATH?=$(HOME)/go
SYSTEM:=
CGO_ENABLED:=0
BUILDOPTS:=-v
BUILDLDFLAGS=-ldflags '-s -w $(EXTRA_LDFLAGS)'

# Print output
WHALE = "+"


.PHONY: all binaries clean docker
all: binaries


FORCE:


# Build a binary from a cmd.
bin/%: cmd/% FORCE
	@echo "$(WHALE) $@${BINARY_SUFFIX}"
	GO111MODULE=on CGO_ENABLED=$(CGO_ENABLED) $(SYSTEM) \
		go build $(BUILDOPTS) -o $@${BINARY_SUFFIX} ${BUILDLDFLAGS} ./$< 


binaries: $(BINARIES)
	@echo "$(WHALE) $@"


clean:
	@echo "$(WHALE) $@"
	@rm -f $(BINARIES)
	@rmdir bin

docker:
	@echo "$(WHALE) $@"
	docker build -t ludns -f Dockerfile.ludns \
		--build-arg github_user=${github_user} --build-arg github_token=${github_token} .
	docker build -t resolvcache -f Dockerfile.resolvcache \
		--build-arg github_user=${github_user} --build-arg github_token=${github_token} .

## Targets for Makefile.release
.PHONY: release
release:
	@$(if $(value BINARY),, $(error Undefined BINARY))
	@$(if $(value COMMAND),, $(error Undefined COMMAND))
	@echo "$(WHALE) $@"
	GO111MODULE=on CGO_ENABLED=$(CGO_ENABLED) $(SYSTEM) \
		    go build $(BUILDOPTS) ${BUILDLDFLAGS} -o $(BINARY) ./cmd/$(COMMAND)

.PHONY: test 
test: 
	@echo "$(WHALE) $@"
