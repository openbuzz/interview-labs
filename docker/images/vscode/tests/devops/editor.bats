#!/usr/bin/env bats

load ../setup_suite

SETTINGS_FILE="/home/user/.local/share/code-server/User/settings.json"

@test "devops extensions are installed" {
  run in_container code-server --list-extensions
  [[ "${status}" -eq 0 ]]
  local lc="${output,,}"
  [[ "${lc}" == *"hashicorp.terraform"* ]]
  [[ "${lc}" == *"ms-kubernetes-tools.vscode-kubernetes-tools"* ]]
}

@test "devops formatter merged without clobbering base settings" {
  run in_container jq -e \
    '.["[terraform]"]["editor.defaultFormatter"] == "hashicorp.terraform"' "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]
  run in_container jq -e \
    '.["[yaml]"]["editor.defaultFormatter"] == "esbenp.prettier-vscode"' "${SETTINGS_FILE}"
  [[ "${status}" -eq 0 ]]
}
