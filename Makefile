.PHONY: all test lint fmt clean deps

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

all: test lint

deps:
	$(GOMOD) download

test:
	$(GOTEST) -v -race -cover ./...

lint:
	golangci-lint run ./...

fmt:
	$(GOCMD) fmt ./...

clean:
	$(GOCLEAN)
