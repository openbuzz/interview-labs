#!/usr/bin/env bats

load ../setup_suite

@test "all devops tools are on the global PATH" {
  run in_container bash -c \
    'for cmd in terraform tofu kubectl helm aws k9s
    do command -v "${cmd}" || exit 1; done'
  [[ "${status}" -eq 0 ]]
}

@test "terraform version matches the devops manifest" {
  local want
  want="$(mise_tool_version devops terraform)"
  [[ -n "${want}" ]]

  run in_container terraform version
  [[ "${status}" -eq 0 ]]
  [[ "${output}" == *"${want}"* ]]
}

@test "kubectl completion loads in an interactive shell" {
  run in_container bash -ic 'complete -p kubectl'
  [[ "${status}" -eq 0 ]]
}

@test "helm and k9s completions load" {
  run in_container bash -ic 'complete -p helm'
  [[ "${status}" -eq 0 ]]
  run in_container bash -ic 'complete -p k9s'
  [[ "${status}" -eq 0 ]]
}

@test "terraform, opentofu, and aws self-completers are registered" {
  run in_container bash -ic 'complete -p terraform'
  [[ "${status}" -eq 0 ]]
  run in_container bash -ic 'complete -p tofu'
  [[ "${status}" -eq 0 ]]
  run in_container bash -ic 'complete -p aws'
  [[ "${status}" -eq 0 ]]
}

@test "extended devops tools are on PATH" {
  run in_container bash -c \
    'command -v tflint && command -v kustomize && command -v trivy && command -v ansible'
  [[ "${status}" -eq 0 ]]
}

@test "ansible community collections are bundled" {
  run in_container bash -c 'ansible-galaxy collection list 2>/dev/null | grep -q community.general'
  [[ "${status}" -eq 0 ]]
}
