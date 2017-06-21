SOURCEDIR := .
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')
# Go utilities
GO_PATH := ${GOPATH}
GO_PATH := $(realpath $(GO_PATH))
GO_LINT := $(GO_PATH)/bin/golint
GO_GODEP := $(GO_PATH)/bin/godep
GO_BINDATA := $(GO_PATH)/bin/bindata
GO_GINKGO := $(GO_PATH)/bin/ginkgo

# Handling project dirs and names
ROOT_DIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
PROJECT_PATH := $(strip $(subst $(GO_PATH)/src/,, $(realpath $(ROOT_DIR))))
PROJECT_NAME := $(lastword $(subst /, , $(PROJECT_PATH)))

BINARY := bin/$(PROJECT_NAME)

TARGETS := $(shell go list ./... | grep -v ^$(PROJECT_PATH)/vendor | sed 's!$(PROJECT_PATH)/!!' | grep -v $(PROJECT_PATH))
TARGETS_LINT := $(patsubst %,lint-%, $(TARGETS))
TARGETS_VET  := $(patsubst %,vet-%, $(TARGETS))
TARGETS_FMT  := $(patsubst %,fmt-%, $(TARGETS))

# Injecting project version and build time
VERSION_GIT := $(shell sh -c 'git describe --always --tags')
BUILD_TIME := `date +%FT%T%z`
VERSION_PACKAGE := $(PROJECT_PATH)/common
LDFLAGS := -ldflags "-X $(VERSION_PACKAGE).Version=${VERSION_GIT} -X $(VERSION_PACKAGE).BuildTime=${BUILD_TIME}"

.DEFAULT_GOAL: $(BINARY)

$(BINARY): $(SOURCES)
	go build ${LDFLAGS} -o ${BINARY} main.go

$(GO_LINT):
	go get -u github.com/golang/lint/golint

$(GO_GODEP):
	go get -u github.com/tools/godep

$(GO_GINKGO):
	go get github.com/onsi/ginkgo/ginkgo

prepare: $(GO_GODEP)
	$(GO_GODEP) restore

install:
	go install ${LDFLAGS} ./...

test-cover: vet $(GO_GINKGO)
	@$(GO_GINKGO) -r --randomizeAllSpecs --randomizeSuites --failOnPending --cover --trace --race --compilers=2

test: vet $(GO_GINKGO)
	@$(GO_GINKGO) -r --randomizeAllSpecs --randomizeSuites --failOnPending --trace --race --compilers=2

vet: $(TARGETS_VET)
# @go vet

$(TARGETS_VET): vet-%: %
	@go vet $</*.go

fmt: $(TARGETS_FMT)
# @go fmt

fmt-check:
	@test -z "$$(gofmt -s -l $(TARGETS) | tee /dev/stderr)"

$(TARGETS_FMT): fmt-%: %
	@gofmt -s -l -w $</*.go

lint: $(GO_LINT) $(TARGETS_LINT)
# @golint

$(TARGETS_LINT): lint-%: %
	@$(GO_LINT) -set_exit_status $<

$(GO_BINDATA):
	go get -u github.com/jteeuwen/go-bindata/...

gen-resources: $(GO_BINDATA)
	$(GO_BINDATA) -o resources/resources.go -pkg resources -prefix resources -ignore resources.go resources/...

clean:
	if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi

.PHONY: test lint vet $(TARGETS_TEST) $(TARGETS_LINT)