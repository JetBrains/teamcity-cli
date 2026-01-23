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
    TC_INSECURE_SKIP_WARN=1 go test -v ./... -timeout 15m -coverprofile=coverage.out -coverpkg=./...

clean:
    rm -rf bin/ .env coverage.out

docs:
    go run scripts/generate-docs.go

generate:
      go generate ./...