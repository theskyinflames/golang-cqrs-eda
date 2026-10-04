# cqrs-eda evals

- `evals.json`, `fixtures/`, `check.py`: quality evals (does Claude follow the
  conventions when the skill is loaded).
- `trigger-evals.json`, `trigger/`: trigger evals (does the skill load when it
  should). See [trigger/README.md](trigger/README.md).

## Running a quality eval

Keep the workspace **outside this repo**, for example
`$TMPDIR/cqrs-eda-workspace`. Runs load the repo as `--plugin-dir`, and Claude
Code refuses edits inside a loaded plugin as sensitive files. A coding eval run
inside the repo writes nothing, and the untouched fixture still passes
`go vet` and `go test`.

`check.py` expects this layout:

```
<iteration>/eval-<id>-<name>/<config>/run-<n>/
  prompt.txt
  outputs/project/      copy of the eval's fixture, where Claude works
  outputs/transcript.jsonl
  checks.json           written by check.py
  grading.json          the other assertions, graded by reading the outputs
```

One run, from `outputs/project`:

```bash
claude -p "$(cat ../../prompt.txt)" \
  --plugin-dir <repo> --model claude-sonnet-5-5 \
  --permission-mode acceptEdits --output-format stream-json --verbose \
  --allowedTools Read Grep Glob Skill Edit Write "Bash(go:*)" "Bash(gofmt:*)" \
    "Bash(grep:*)" "Bash(find:*)" "Bash(ls:*)" "Bash(cat:*)" "Bash(mkdir:*)" \
    "Bash(bash:*)" "Bash(sh:*)" \
  > ../transcript.jsonl
```

The review eval (3) needs only the read-only tools. Then run
`python3 evals/cqrs-eda/check.py <iteration>` for the mechanical checks.
