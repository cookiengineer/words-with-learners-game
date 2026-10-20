.PHONY: run build test fmt vet check clean

run:
	go run ./cmd/server

build:
	mkdir -p bin
	go build -o bin/words-with-learners ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './bin/*')

vet:
	go vet ./...

check: fmt vet test

clean:
	rm -rf bin
