# TeamCity CLI
#
#   just build   - build to bin/tc
#   just install - go install
#   just unit    - fast tests (no TeamCity required)
#   just test    - full test suite with local TeamCity in Docker
#   just local   - start local TeamCity and create .env
#   just stop    - stop local TeamCity
#   just clean   - stop, remove volumes, and clean artifacts
#   just docs    - generate CLI documentation

build:
    go build -o bin/tc ./tc

install:
    go install ./tc

# Fast unit tests (no TeamCity required)
unit:
    TC_INSECURE_SKIP_WARN=1 go test -v ./internal/config ./internal/errors ./internal/output

# Full test suite with local TeamCity
test:
    #!/usr/bin/env bash
    set -e
    go run scripts/setup-local-teamcity.go
    . .env
    TC_INSECURE_SKIP_WARN=1 go test -v -json ./... -timeout 0 -coverpkg=./... -coverprofile=coverage.out | tparse -all -follow

# CI test suite (no tparse, includes cleanup)
test-ci:
    #!/usr/bin/env bash
    set -e
    go run scripts/setup-local-teamcity.go
    . .env
    TC_INSECURE_SKIP_WARN=1 go test -v -json ./... -timeout 0 -coverpkg=./... -coverprofile=coverage.out
    docker compose down -v

# Start local TeamCity and create .env
local:
    go run scripts/setup-local-teamcity.go

stop:
    docker compose down

clean:
    docker compose down -v
    rm -rf bin/ .env coverage.out

docs:
    go run scripts/generate-docs.go
