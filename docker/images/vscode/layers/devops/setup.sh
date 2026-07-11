#!/usr/bin/env bash

# Wire bash completion for the devops mise tools, register the IaC + Kubernetes
# editor extensions, and merge the devops settings.

set -e -u -o pipefail

# Tools whose own "completion bash" subcommand emits a completion script.
for tool in kubectl helm k9s; do
  "${tool}" completion bash >"/etc/bash_completion.d/${tool}"
done

# Tools that act as their own completer via "complete -C".
{
  printf 'complete -C terraform terraform\n'
  printf 'complete -C tofu tofu\n'
  printf 'complete -C aws_completer aws\n'
} >/etc/bash_completion.d/mise-tools

# Ansible: config management (Python tool via uv; base provides uv + Python).
# System dirs so the bins (ansible, ansible-playbook, …) land on the global PATH.
# ponytail: ansible-core is primary so the `ansible` console_scripts land on
# PATH; --with ansible adds the full community collection bundle into the same
# venv (ansible-core discovers the collections there).
export UV_TOOL_BIN_DIR=/usr/local/bin
export UV_TOOL_DIR=/opt/uv/tools
uv tool install --quiet "ansible-core==2.21.1" --with "ansible==14.0.0"

extensions=(
  hashicorp.terraform                         # HashiCorp Terraform — HCL + language server
  ms-kubernetes-tools.vscode-kubernetes-tools # Kubernetes — manifests & cluster tooling
)
cs-extension-register "${extensions[@]}"
cs-settings-merge /tmp/profile.settings.json
