SOURCEDIR := .
ifeq ($(OS),Windows_NT)
	SOURCES := $(shell dir /a-d /b /s *.go)
else
	SOURCES := $(shell find $(SOURCEDIR) -name '*.go')
endif
# Go utilities
ifeq ($(OS),Windows_NT)
	GO_PATH := $(subst \,/,${GOPATH})
else
	GO_PATH := ${GOPATH}
endif

# GO_PATH := $(realpath $(GO_PATH))
ifeq ($(OS),Windows_NT)
	BINARY_EXT := .exe
else
	BINARY_EXT :=
endif
GO_LINT := $(GO_PATH)/bin/golint$(BINARY_EXT)
GO_GLIDE := $(GO_PATH)/bin/glide$(BINARY_EXT)
GO_BINDATA := $(GO_PATH)/bin/bindata$(BINARY_EXT)
GO_GINKGO := $(GO_PATH)/bin/ginkgo$(BINARY_EXT)

# Handling project dirs and names
ROOT_DIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
ifeq ($(OS),Windows_NT)
	ROOT_DIR := $(subst \,/,${ROOT_DIR})
endif
PROJECT_PATH := $(strip $(subst $(GO_PATH)/src/,, $(ROOT_DIR)))
PROJECT_PATH := $(patsubst %/,%, $(PROJECT_PATH))
PROJECT_NAME := $(lastword $(subst /, , $(PROJECT_PATH)))

BINARY := bin/$(PROJECT_NAME)$(BINARY_EXT)

TARGETS := $(shell go list ./... | grep -v ^$(PROJECT_PATH)/vendor | sed s!$(PROJECT_PATH)/!! | grep -v $(PROJECT_PATH))
TARGETS_LINT := $(patsubst %,lint-%, $(TARGETS))
TARGETS_VET  := $(patsubst %,vet-%, $(TARGETS))
TARGETS_FMT  := $(patsubst %,fmt-%, $(TARGETS))

# Injecting project version and build time
ifeq ($(OS),Windows_NT)
	VERSION_GIT := $(shell cmd /C 'git describe --always --tags')
else
	VERSION_GIT := $(shell sh -c 'git describe --always --tags')
endif

ifeq ($(VERSION_GIT),)
	VERSION_GIT = "vv0.7.0"
endif

ifeq ($(OS),Windows_NT)
	BUILD_TIME := $(shell PowerShell -Command "get-date -format yyyy-MM-ddTHH:mm:SSzzz")
else
	BUILD_TIME := `date +%FT%T%z`
endif
VERSION_PACKAGE := main
LDFLAGS := -ldflags "-X $(VERSION_PACKAGE).common.Version=${VERSION_GIT} -X $(VERSION_PACKAGE).common.BuildTime=${BUILD_TIME}"

.DEFAULT_GOAL: $(BINARY)

$(BINARY): $(SOURCES)
	go build ${LDFLAGS} -o ${BINARY} main.go

$(GO_LINT):
	go get -u github.com/golang/lint/golint

$(GO_GLIDE):
	go get -u github.com/Masterminds/glide

$(GO_GINKGO):
	go get github.com/onsi/ginkgo/ginkgo
	go get github.com/onsi/gomega

prepare: $(GO_GLIDE)
	$(GO_GLIDE) install

install:
	go install ${LDFLAGS}

test-cover: vet $(GO_GINKGO)
	@$(GO_GINKGO) -r --randomizeAllSpecs --randomizeSuites --failOnPending --cover --trace --race --compilers=2

test: vet $(GO_GINKGO)
	@$(GO_GINKGO) -r --randomizeAllSpecs --randomizeSuites --failOnPending --trace --race --compilers=2

vet: $(TARGETS_VET)
# @go vet

$(TARGETS_VET): vet-%: %
	@go vet $(PROJECT_PATH)/$</

fmt-check:
	@test -z "$(gofmt -s -l $(TARGETS) | tee /dev/stderr)"

fmt: $(TARGETS_FMT)
# @go fmt

$(TARGETS_FMT): fmt-%: %
	@gofmt -s -w $</

lint: $(GO_LINT) $(TARGETS_LINT)
# @golint

$(TARGETS_LINT): lint-%: %
	@$(GO_LINT) $(PROJECT_PATH)/$<

$(GO_BINDATA):
	go get -u github.com/jteeuwen/go-bindata/...

gen-resources: $(GO_BINDATA)
	$(GO_BINDATA) -o resources/resources.go -pkg resources -prefix resources -ignore resources.go resources/...

clean:
	test -f ${BINARY} && exit 0
	$(if $(filter $(OS),Windows_NT), del $(subst /,\,${BINARY}), rm ${BINARY})

.PHONY: test lint vet $(TARGETS_TEST) $(TARGETS_LINT)

include Makefile.docker