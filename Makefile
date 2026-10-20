.PHONY: run build test fmt vet check clean

BINARY  := bin/words-with-learners
GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)
LDFLAGS := -s -w

run:
	go run ./cmd/server

build:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './bin/*')

vet:
	go vet ./...

check: fmt vet test

clean:
	rm -rf bin
