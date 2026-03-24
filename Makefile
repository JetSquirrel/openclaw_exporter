MAKEFILE_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

.PHONY: build build-linux build-windows build-darwin vet lint test clean run dev

build:
	CGO_ENABLED=0 go build -o openclaw_exporter .

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o openclaw_exporter .

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o openclaw_exporter .

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o openclaw_exporter.exe .

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test -race -count=1 ./...

clean:
	rm -f openclaw_exporter openclaw_exporter.exe

run: build
	./openclaw_exporter -openclaw.dir=$(HOME)/.openclaw/workspace

dev: build
	@echo "Starting dev mode (watching for changes)..."
	@trap 'kill $$PID 2>/dev/null; exit 0' INT TERM; \
	while true; do \
		./openclaw_exporter -openclaw.dir=$(HOME)/.openclaw/workspace & PID=$$!; \
		fswatch -1 -r --include='\.go$$' --exclude='.*' $(MAKEFILE_DIR); \
		echo "Change detected, rebuilding..."; \
		kill $$PID 2>/dev/null; wait $$PID 2>/dev/null; \
		$(MAKE) build || continue; \
	done
