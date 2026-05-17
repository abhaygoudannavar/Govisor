.PHONY: build clean test

build:
	go build -o bin/govisord ./cmd/govisord
	go build -o bin/govisor ./cmd/govisor

clean:
	rm -rf bin/

test:
	go test ./internal/... -v
