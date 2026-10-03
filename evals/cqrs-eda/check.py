#!/usr/bin/env python3
"""Mechanical checks for cqrs-eda evals. Usage: check.py <iteration-dir>

Writes checks.json next to each run's outputs/ with the results of the
assertions that can be verified by running commands. The rest are graded by
reading the outputs.
"""
import json
import pathlib
import subprocess
import sys

FIXTURES = pathlib.Path(__file__).parent / "fixtures"


def run(cmd, cwd):
    p = subprocess.run(cmd, cwd=cwd, shell=True, capture_output=True, text=True)
    return p.returncode == 0, (p.stdout + p.stderr)[-1500:]


def go_ok(proj):
    ok_vet, out_vet = run("go vet ./...", proj)
    ok_test, out_test = run("go test ./...", proj)
    return ok_vet and ok_test, f"vet: {'ok' if ok_vet else out_vet}\ntest: {'ok' if ok_test else out_test}"


def checks_for(eval_name, proj):
    res = {}
    if eval_name == "orders-from-skeleton":
        res["1.11"] = go_ok(proj)
    elif eval_name == "new-invoicing-service":
        gomod = proj / "go.mod"
        mod = gomod.read_text() if gomod.exists() else ""
        res["2.1"] = ("module github.com/acme/invoicing" in mod, mod.splitlines()[0] if mod else "no go.mod")
        plat = proj / "internal" / "platform"
        present = [p for p in ("bus", "cqrs", "events", "ddd") if (plat / p).is_dir()]
        ok, out = run("grep -rh 'internal/platform/' --include=*.go internal/platform | sort -u | head", proj)
        rewritten = bool(present) and "example.com/app" not in out
        res["2.2"] = (len(present) == 4 and rewritten, f"present={present}\n{out}")
        res["2.11"] = go_ok(proj)
    elif eval_name == "review-billing-subtle":
        ok, out = run(f"diff -r -q {FIXTURES / 'billing-subtle'} .", proj)
        res["3.10"] = (ok, out or "identical to fixture")
    return {k: {"passed": v[0], "evidence": v[1]} for k, v in res.items()}


def main():
    it = pathlib.Path(sys.argv[1])
    # Layout: <iteration>/eval-<id>-<name>/<config>/run-<n>/outputs
    for run_dir in sorted(it.glob("eval-*/*/run-*/outputs")):
        eval_name = run_dir.parts[-4].split("-", 2)[2]
        cfg = run_dir.parts[-3]
        res = checks_for(eval_name, run_dir / "project")
        (run_dir.parent / "checks.json").write_text(json.dumps(res, indent=2))
        print(eval_name, cfg, {k: v["passed"] for k, v in res.items()})


if __name__ == "__main__":
    main()
