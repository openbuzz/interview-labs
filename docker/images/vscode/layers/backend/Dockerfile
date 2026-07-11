# Backend profile — backend engineer toolchain on top of the parent image.
# `parent` is supplied by bake (contexts = { parent = "target:base" }).

# trivy:ignore:DS-0001 — parent is a bake named context, not a registry ref; no tag applies
FROM parent AS backend
USER root
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ARG DEBIAN_FRONTEND=noninteractive

# Install build-essential: C toolchain for Go and CGO libraries.
RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    rm -rf /var/lib/apt/lists/*

COPY mise.toml /etc/mise/config.toml
RUN mise install && \
    rm -rf /home/user/go /home/user/.cache/go-build && \
    chown -R user:user /home/user

# Register the Go extension, merge backend editor settings, install JS/TS tooling.
COPY settings.json /tmp/settings.json
RUN <<'EOF'
set -e -u -o pipefail

cs-extension-register golang.go
cs-settings-merge /tmp/settings.json
rm -f /tmp/settings.json

corepack enable
corepack prepare "pnpm@11.9.0" "yarn@4.17.0" --activate
npm install -g "typescript@6.0.3"
EOF

USER user
RUN cs-extension-install
