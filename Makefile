SHELL := /usr/bin/env bash

BUILD_TYPE  ?= release

.PHONY: build test

build: clean
	@chmod +x ./build_lib.sh
	./build_lib.sh $(BUILD_TYPE)

test:
	TURSO_GO_NOCACHE=1 go test -v ./...

cache-print:
	scripts/cache.sh print

cache-clean:
	scripts/cache.sh clean

