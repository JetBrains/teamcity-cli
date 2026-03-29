FROM node:22-slim

RUN npm install -g @anthropic-ai/claude-code@latest

# Install teamcity CLI
COPY bin/teamcity /usr/local/bin/teamcity
RUN chmod +x /usr/local/bin/teamcity

RUN groupadd -r bench && useradd -r -g bench -m bench
USER bench
WORKDIR /workspace

ENTRYPOINT ["claude"]
