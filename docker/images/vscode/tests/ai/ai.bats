#!/usr/bin/env bats

# Runs against both -ai images; AI_PARENT (backend|devops) selects the
# parent-specific assertions.

load ../setup_suite

@test "AI assistant CLIs launch" {
  run in_container bash -c 'claude --version && codex --version && opencode --version'
  [[ "${status}" -eq 0 ]]
  [[ -n "${output}" ]]
}

@test "AI config files are present and owned by user" {
  run in_container stat -c '%U' \
    /home/user/.claude.json \
    /home/user/.codex/config.toml \
    /home/user/.config/opencode/opencode.json
  [[ "${status}" -eq 0 ]]
  [[ "${output}" != *"root"* ]]
}

@test "no API key is baked into the image" {
  # ponytail: unanchored pattern matched "sk-" mid-word in vendored bundles
  # (marketplace extension assets, ansible/uv package caches) with no real
  # secret present; \b + excluding those trees keeps the check meaningful.
  run in_container bash -c \
    'grep -rniE --exclude-dir=extensions --exclude-dir=.cache \
      "\bsk-(or-v1-|ant-|proj-)?[a-zA-Z0-9_-]{20,}" /home/user /root /etc 2>/dev/null || true'
  [[ "${output}" == "" ]]
}

@test "no credential env vars are baked into the image" {
  run in_container bash -c 'printenv ANTHROPIC_AUTH_TOKEN'
  [[ "${status}" -ne 0 ]]
  run in_container bash -c 'printenv ANTHROPIC_API_KEY'
  [[ "${status}" -ne 0 ]]
}

@test "parent profile tooling is still present (inherited)" {
  : "${AI_PARENT:?AI_PARENT must be set (backend|devops)}"
  if [[ "${AI_PARENT}" == "backend" ]]; then
    run in_container bash -c 'command -v go && command -v pnpm && command -v tsc'
  else
    run in_container bash -c \
      'command -v terraform && command -v kubectl && command -v trivy && command -v ansible'
  fi
  [[ "${status}" -eq 0 ]]
}

@test "Claude Code OpenRouter routing env is set" {
  run in_container bash -c \
    'printf "%s|%s" "${ANTHROPIC_BASE_URL}" "${ANTHROPIC_MODEL}"'
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == "https://openrouter.ai/api|anthropic/"* ]]
}
