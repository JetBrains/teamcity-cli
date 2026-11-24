# Base image for Harbor runs of the eval tasks: the CLI, the shared checks and the verifier.
# Build from the repo root with `just eval-harbor-image`; each task's environment/Dockerfile starts FROM it.
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends python3 curl ca-certificates git jq \
    && rm -rf /var/lib/apt/lists/*

COPY bin/teamcity-linux /usr/local/bin/teamcity
COPY evals/checks.py evals/grade.py evals/cli_schema.json evals/harbor/verify.py /opt/tc-eval/
COPY evals/scaffold/events.py evals/scaffold/runner.py /opt/tc-eval/scaffold/
COPY evals/harbor/entrypoint.sh /usr/local/bin/tc-eval-entrypoint

WORKDIR /app
ENTRYPOINT ["/usr/local/bin/tc-eval-entrypoint"]
CMD ["sleep", "infinity"]
