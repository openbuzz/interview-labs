# AI layer — AI assistant CLIs on top of a parent profile.
# `parent` is supplied by bake (contexts = { parent = "target:<profile>" }).

# trivy:ignore:DS-0001 — parent is a bake named context, not a registry ref; no tag applies
FROM parent AS ai
USER root
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# AI assistant CLIs — pinned.
RUN npm install -g \
      "@anthropic-ai/claude-code@2.1.197" \
      "@openai/codex@0.142.4" \
      "opencode-ai@1.17.11"

# Config — credentials arrive at runtime; never bake a key.
COPY files /tmp/config
RUN install -d -o user -g user \
      /home/user/.codex /home/user/.config/opencode /home/user/.claude && \
    install -m 0644 -o user -g user \
      /tmp/config/claude.json /home/user/.claude.json && \
    install -m 0644 -o user -g user \
      /tmp/config/codex.toml /home/user/.codex/config.toml && \
    install -m 0644 -o user -g user \
      /tmp/config/opencode.json /home/user/.config/opencode/opencode.json && \
    install -m 0644 -o user -g user \
      /tmp/config/settings.json /home/user/.claude/settings.json && \
    rm -rf /tmp/config

# Static AI runtime config — non-secret; credentials arrive at runtime.
ENV ANTHROPIC_BASE_URL=https://openrouter.ai/api
ENV ANTHROPIC_MODEL=anthropic/claude-sonnet-5

# Disable Claude Code telemetry, error reporting, and the auto-updater.
ENV CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1

# Disable opencode's automatic update check (https://opencode.ai/docs/cli/).
# Codex has no documented telemetry/update-check env var (only config.toml
# keys: check_for_update_on_startup, analytics.enabled, feedback.enabled) —
# DO_NOT_TRACK plus the no-baked-key posture is its coverage.
ENV OPENCODE_DISABLE_AUTOUPDATE=1

# Register the AI editor extensions (installed as user below).
RUN <<'EOF'
set -e -u -o pipefail
cs-extension-register anthropic.claude-code openai.chatgpt sst-dev.opencode
EOF

# Fix home ownership — root build steps above leave root-owned paths under
# /home/user (e.g. the OpenCode data dir); mirror the base layer.
RUN chown -R user:user /home/user

USER user
RUN cs-extension-install
