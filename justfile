# TeamCity CLI

build:
    go build -o bin/tc ./tc

lint:
    go fmt ./... && golangci-lint run --tests=false ./...

install:
    go install ./tc

unit:
    TC_INSECURE_SKIP_WARN=1 go test -v ./internal/config ./internal/errors ./internal/output ./internal/cmd

test:
    TC_INSECURE_SKIP_WARN=1 go test -v ./... -timeout 15m -tags=integration -coverprofile=coverage.out -coverpkg=./...

clean:
    rm -rf bin/ dist/ .env coverage.out

docs:
    go run scripts/generate-docs.go

generate:
    go generate ./...

snapshot:
    goreleaser release --snapshot --clean --skip=publish

release-dry-run:
    goreleaser release --clean --skip=publish

release:
    goreleaser release --clean