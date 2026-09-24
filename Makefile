.PHONY: build install test vet

build:
	mkdir -p dist
	go build -o ./dist/mysql-stash

install:
	go install

test:
	go test ./... -v

vet:
	go vet ./...
