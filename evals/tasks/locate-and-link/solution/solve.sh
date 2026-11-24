#!/bin/sh
# Oracle: find the project by name, then bind the repository to it.
set -e
project="$(teamcity project list --json --limit 0 | jq -r '.project[] | select(.name == "Backend") | .id')"
teamcity link --project "$project"
