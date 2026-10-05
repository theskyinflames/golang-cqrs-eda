# golang-cqrs-eda

A Claude Code plugin that teaches Claude to build Go services with CQRS,
event-driven architecture and DDD. It is based on
[theskyinflames/cqrs-eda](https://github.com/theskyinflames/cqrs-eda).

Instead of importing a library, Claude copies a small platform into your
project (`internal/platform/{bus,cqrs,events,ddd}`, ~400 lines, no external
dependencies). After that, the code is yours to change.

## Install

In Claude Code:

```
/plugin marketplace add theskyinflames/golang-cqrs-eda
/plugin install cqrs-eda@theskyinflames
```

Requires Go 1.24+. On Go 1.24–1.26 the platform uses `github.com/google/uuid`;
from Go 1.27 it uses the stdlib `uuid`.

## What you get

- **`cqrs-eda` skill.** Loads when you work on a Go service with commands,
  queries, events, aggregates or policies, or ask for a new one. Claude then
  follows the conventions: every use case is a command or query, one aggregate
  per command, business rules in the domain, policies that only turn events into
  commands, and the middleware order logging → publish events → unit of work.
  If your project doesn't have the platform yet, Claude asks before installing it.
- **`cqrs-reviewer` agent.** A read-only review of a package or a whole
  service. It reports runtime bugs first (e.g. handlers that never dispatch,
  events registered twice, charges that run twice), then design issues, each
  with file:line and a fix. Ask Claude to use the cqrs-reviewer agent, or let
  the skill delegate large reviews to it.

Just describe the task, for example:

> add an endpoint to cancel an order; shipping should react when it happens

> start a new Go service for invoicing with create, pay and list unpaid

> review internal/billing before we ship it

## Repository layout

| Path | Contents |
|---|---|
| `skills/cqrs-eda/` | The skill: `SKILL.md`, platform template, `install.sh`, `references/patterns.md` |
| `agents/cqrs-reviewer.md` | The reviewer agent |
| `dev/` | `sample-shop`, the compiled example that `gen-patterns.sh` turns into `patterns.md` |
| `evals/cqrs-eda/` | Quality and trigger evals, and how to run them |
| `STATUS.md` | Progress and eval results |

## License

MIT. The original `cqrs-eda` repository is still GPL-3.0.
