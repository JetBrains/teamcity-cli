# TeamCity CLI
#
# Testing:
#   Remote server: Configure .env with your TEAMCITY_URL and TEAMCITY_TOKEN
#   Local Docker:  Run `just local`, which creates .env with local config
#
# Tests always use .env - `just local` overwrites it with local Docker config

build:
    go build -o bin/tc ./tc

install PREFIX="$HOME/go/bin":
    go build -o {{PREFIX}}/tc ./tc

test:
    TC_INSECURE_SKIP_WARN=1 go test -v -json ./... -timeout 0 -coverpkg=./... -coverprofile=coverage.out | tparse -all -follow

# Start local TeamCity in Docker and configure .env for testing
local:
    go run scripts/setup-local-teamcity.go

local-stop:
    docker compose down

local-reset:
    docker compose down -v

docs:
    go run scripts/generate-docs.go

docs-check:
    go run scripts/generate-docs.go --check

clean:
    rm -rf bin/
