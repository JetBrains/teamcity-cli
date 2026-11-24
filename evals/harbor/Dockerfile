# Base image for Harbor runs of the eval tasks: the CLI, the shared checks and the verifier.
# Built by run-harbor.sh and validate-tasks.sh; each task's environment/Dockerfile starts FROM it.
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends python3 curl ca-certificates git jq \
    && rm -rf /var/lib/apt/lists/*

COPY bin/teamcity-linux /usr/local/bin/teamcity
COPY evals/checks.py evals/grade.py evals/cli_schema.json evals/harbor/verify.py /opt/tc-eval/
COPY evals/scaffold/events.py evals/scaffold/runner.py /opt/tc-eval/scaffold/
COPY evals/harbor/entrypoint.sh /usr/local/bin/tc-eval-entrypoint

# The CLI is logged in through its config file, the way most users are. Nothing about the server
# goes into the environment, where an agent would find the token and skip the CLI.
ARG TC_EVAL_URL
ARG TC_EVAL_TOKEN
RUN mkdir -p /root/.config/tc && printf 'analytics_notice_shown: true\ndefault_server: %s\nservers:\n    %s:\n        token: %s\n' \
    "$TC_EVAL_URL" "$TC_EVAL_URL" "$TC_EVAL_TOKEN" > /root/.config/tc/config.yml && chmod 600 /root/.config/tc/config.yml

WORKDIR /app
ENTRYPOINT ["/usr/local/bin/tc-eval-entrypoint"]
CMD ["sleep", "infinity"]
