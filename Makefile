# Variables
GOCMD = go
GOBUILD_ENVS = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
BINARY_NAME = kapprofiler
GOFILES = $(shell find . -type f -name '*.go')

$(BINARY_NAME): $(GOFILES) go.mod go.sum
	go build -o $(BINARY_NAME) -v
	# CGO_ENABLED=0 go build -tags osusergo,netgo -ldflags="-extldflags=-static" -o wlftracer wl-file-activity-tracer.go

install: $(BINARY_NAME)
	./scripts/install-in-pod.sh $(BINARY_NAME)

open-shell:
	./scripts/open-shell-in-pod.sh

deploy-dev-pod:
	kubectl apply -f dev/devpod.yaml

clean:
	rm -f $(BINARY_NAME)

all: $(BINARY_NAME)

.PHONY: clean all install deploy-dev-pod