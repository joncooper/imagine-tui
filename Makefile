.PHONY: build test lint golden-update clean

build:
	go build ./cmd/imagine-tui

test:
	go test ./...

lint:
	golangci-lint run

golden-update:
	GOLDEN_UPDATE=1 go test ./internal/widget/...

clean:
	rm -f imagine-tui
	go clean ./...
