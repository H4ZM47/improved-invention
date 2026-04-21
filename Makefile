.PHONY: fmt lint test build run

fmt:
	./scripts/fmt.sh

lint:
	./scripts/lint.sh

test:
	./scripts/test.sh

build:
	./scripts/build.sh

run:
	go run ./cmd/task
