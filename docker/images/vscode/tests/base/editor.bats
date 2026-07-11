#!/usr/bin/env bats

load ../setup_suite

SETTINGS_FILE="/home/user/.local/share/code-server/User/settings.json"

@test "base extensions are installed" {
  run in_container code-server --list-extensions
  [[ "${status}" -eq 0 ]]
  local lc="${output,,}"
  for ext in \
    pkief.material-icon-theme \
    redhat.vscode-yaml \
    ms-python.python \
    esbenp.prettier-vscode \
    charliermarsh.ruff \
    ms-azuretools.vscode-containers; do
    [[ "${lc}" == *"${ext}"* ]]
  done
}

@test "the extension manifest is removed after install" {
  run in_container test -e /var/run/code-server/extensions.list
  [[ "${status}" -ne 0 ]]
}

@test "settings.json is valid json" {
  run in_container jq -e . "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]
}

@test "base formatter + interpreter settings are present" {
  run in_container jq -e \
    '.["[yaml]"]["editor.defaultFormatter"] == "esbenp.prettier-vscode"' "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]
  run in_container jq -e \
    '.["[markdown]"]["editor.defaultFormatter"] == "esbenp.prettier-vscode"' "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]
  run in_container jq -e \
    '.["[python]"]["editor.defaultFormatter"] == "charliermarsh.ruff"' "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]
  run in_container jq -e \
    '."python.defaultInterpreterPath" == "/usr/local/bin/python3"' "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]
}
