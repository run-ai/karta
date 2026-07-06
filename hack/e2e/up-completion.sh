# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Tab-completion for hack/e2e/up.sh: completes operator names (plus "all",
# --list, --help) so you can pick several with completion, e.g.
#   ./hack/e2e/up.sh dynamo <TAB>      -> next operator
# The operator list is read from up.sh itself, so it stays in sync.
#
# Enable it by sourcing this file (add the line to your ~/.zshrc or ~/.bashrc):
#   source hack/e2e/up-completion.sh
# shellcheck shell=bash

# zsh: turn on bash-style completion so the same function works in both shells.
if [ -n "${ZSH_VERSION:-}" ]; then
  autoload -U +X bashcompinit 2>/dev/null && bashcompinit 2>/dev/null
fi

_karta_e2e_up_complete() {
  local cmd="${COMP_WORDS[0]}" cur="${COMP_WORDS[COMP_CWORD]}"
  local script ops
  # Resolve the up.sh being completed and read its operator list.
  script="$(command -v "${cmd}" 2>/dev/null || echo "${cmd}")"
  ops="$(sed -n 's/^ALL_WORKLOADS=(\(.*\))/\1/p' "${script}" 2>/dev/null)"
  # shellcheck disable=SC2207  # splitting the operator list into words is intended
  COMPREPLY=($(compgen -W "${ops} all --list --help" -- "${cur}"))
}

complete -F _karta_e2e_up_complete up.sh ./hack/e2e/up.sh hack/e2e/up.sh 2>/dev/null || true
