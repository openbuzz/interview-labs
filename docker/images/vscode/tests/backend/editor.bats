#!/usr/bin/env bats

load ../setup_suite

SETTINGS_FILE="/home/user/.local/share/code-server/User/settings.json"

@test "the Go extension is installed" {
  run in_container code-server --list-extensions
  [[ "${status}" -eq 0 ]]
  [[ "${output,,}" == *"golang.go"* ]]
}

@test "backend go formatter merged without clobbering base settings" {
  run in_container jq -e '.["[go]"]["editor.defaultFormatter"] == "golang.go"' "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]

  run in_container jq -e \
    '.["[yaml]"]["editor.defaultFormatter"] == "esbenp.prettier-vscode"' "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]
}
