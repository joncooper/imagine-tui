.PHONY: build test lint golden-update clean install-hooks

build:
	go build ./cmd/imagine-tui

test:
	go test ./...

lint:
	golangci-lint run

golden-update:
	GOLDEN_UPDATE=1 go test ./internal/widget/...

install-hooks:
	git config core.hooksPath .githooks

clean:
	rm -f imagine-tui
	go clean ./...
