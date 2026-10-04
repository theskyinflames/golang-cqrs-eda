# Trigger eval

Measures whether Claude loads the `cqrs-eda` skill for the queries in
`../trigger-evals.json` (10 that should trigger, 10 near-misses), and can run
the skill-creator loop that rewrites the skill description.

```sh
# Measure one description (the current SKILL.md one, or --description "...")
evals/cqrs-eda/trigger/run.sh eval --model claude-sonnet-5-5 --verbose

# Tune the description
evals/cqrs-eda/trigger/run.sh loop --model claude-sonnet-5-5 --max-iterations 5 \
  --num-workers 5 --results-dir <dir> --report <dir>/report.html --verbose
```

`run.sh` copies `../fixtures/shop-skeleton` to
`$TMPDIR/cqrs-eda-trigger-workspace` (override with `TRIGGER_WORKSPACE`) and
runs `claude -p` there, so queries see a Go service and the temporary skill
copies stay out of `~/.claude/commands`.

## Origin

`scripts/` comes from Anthropic's skill-creator skill (Apache-2.0, see
`LICENSE.txt`). Only the modules the trigger eval needs are kept.
`run_eval.py` and `run_loop.py` are modified:

- **The skill counts as triggered if it loads at any point in the run.**
  Upstream only checks the first tool call, so a query where Claude reads repo
  files before loading the skill counted as a miss.
- **A run stops after 8 tool calls that don't load the skill**
  (`MAX_TOOL_CALLS`), or as soon as it loads, to bound cost.
- **The default timeout per query is 90 s**, not 30 s.

Upstream also picks the project root by walking up from the current directory
to the first `.claude/`. Run from inside `~/.claude`, that resolves to `~`:
queries run without any Go code and the temporary skills land in
`~/.claude/commands`. `run.sh` avoids this by running from the workspace.
