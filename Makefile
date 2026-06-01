.PHONY: test test-short test-e2e vet build fmt all staticcheck gopls-check lint check tools

all: vet build test-short

test:
	go test ./...

test-short:
	go test ./... -short

test-e2e:
	go test -tags=e2e ./e2e/rpg/...

vet:
	go vet ./...

build:
	go build ./...
	go build -o bin/rpg-cli ./cmd/rpg-cli
	go build -o bin/rpg-server ./cmd/rpg-server

fmt:
	gofmt -w ./agent ./cmd ./e2e ./internal ./rpg ./world

staticcheck:
	staticcheck ./...

gopls-check:
	find ./agent ./cmd ./e2e ./internal ./rpg ./world -name '*.go' -not -path '*/testdata/*' -print0 | xargs -0 gopls check

lint: vet staticcheck

check: lint gopls-check

tools:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/tools/gopls@latest
