#!/usr/bin/env bash
# Runs the trigger eval (or the description loop) for the cqrs-eda skill
# inside an isolated copy of the shop-skeleton fixture.
#
#   evals/cqrs-eda/trigger/run.sh eval --model claude-sonnet-5-5 [--description "..."]
#   evals/cqrs-eda/trigger/run.sh loop --model claude-sonnet-5-5 --max-iterations 5
#
# Extra arguments go to scripts.run_eval or scripts.run_loop unchanged.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
evals="$(dirname "$here")"
repo="$(cd "$evals/../.." && pwd)"
# Outside the repo, so claude -p does not pick up this repo's CLAUDE.md or .claude/.
workspace="${TRIGGER_WORKSPACE:-${TMPDIR:-/tmp}/cqrs-eda-trigger-workspace}"

case "${1:-}" in
  eval) module=scripts.run_eval ;;
  loop) module=scripts.run_loop ;;
  *) echo "usage: $0 eval|loop [args...]" >&2; exit 2 ;;
esac
shift

# claude -p runs in this project, so queries see Go code and the temporary
# skill copies go to its .claude/commands, not ~/.claude/commands.
rm -rf "$workspace"
cp -R "$evals/fixtures/shop-skeleton" "$workspace"
mkdir -p "$workspace/.claude"
git -C "$workspace" init -q
git -C "$workspace" add -A
git -C "$workspace" -c user.name=eval -c user.email=eval@localhost commit -qm fixture

cd "$workspace"
PYTHONPATH="$here" python3 -m "$module" \
  --eval-set "$evals/trigger-evals.json" \
  --skill-path "$repo/skills/cqrs-eda" \
  "$@"
