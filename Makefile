# Variables
GOCMD = go
GOBUILD_ENVS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOTEST_SUDO_PREFIX = sudo --preserve-env=HOME --preserve-env=GOPATH
GOGET = $(GOCMD) get
BINARY_NAME = kapprofiler
GOFILES = $(shell find . -type f -name '*.go')

$(BINARY_NAME): $(GOFILES) go.mod go.sum Makefile
	CGO_ENABLED=0 go build -o $(BINARY_NAME) -v

test:
	$(GOTEST_SUDO_PREFIX) $(GOTEST) -v ./...

install: $(BINARY_NAME)
	./scripts/install-in-pod.sh $(BINARY_NAME)

open-shell:
	./scripts/open-shell-in-pod.sh

deploy-dev-pod:
	kubectl apply -f dev/devpod.yaml

build: $(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)

all: $(BINARY_NAME)

.PHONY: clean all install deploy-dev-pod test open-shell build