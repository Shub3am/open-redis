PORT ?= 6379

.PHONY: build run test bench vet fmt check

build:
	go build -o bin/open-redis ./cmd/open-redis

run: build
	./bin/open-redis -port $(PORT)

test:
	go test -race ./...

bench:
	go test -run '^$$' -bench . -benchmem ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

check: vet test
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)
