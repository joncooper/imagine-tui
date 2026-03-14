SOCKET ?= /tmp/imagine-tui.sock
LOG ?= /tmp/imagine-tui.log

.PHONY: build serve test lint golden-update clean install-hooks

build:
	go build ./cmd/imagine-tui

serve: build
	./imagine-tui serve --socket $(SOCKET) --log $(LOG)

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
