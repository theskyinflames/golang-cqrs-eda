---
name: cqrs-eda
description: CQRS, event-driven (EDA) and DDD conventions for Go services, with copy-in platform packages (command/query/event buses, typed generic handlers, middleware for logging, transactions and event publishing, aggregate roots). Use this skill whenever working on a Go service that has, or should have, commands, queries, command/query handlers, domain events, event handlers or policies, aggregates, a command bus or event bus, or a unit of work — including when the user just asks to "add an endpoint that creates/updates X", "react when Y happens", "add a use case", or "set up a new Go service" with DDD/hexagonal structure, even if CQRS is never named. Also use it to review Go code for CQRS/EDA violations.
---

# CQRS / EDA for Go

This skill gives a Go project a small, owned CQRS/EDA platform and the rules
for building on it. The platform is **copied into the project** (copy-in
model), not imported: the project owns the code afterwards.

## 1. Detect or install the platform

Check whether the project already has it:

```bash
grep -rl --include=*.go 'internal/platform/cqrs"' . | head -1
```

- **Found** → use it as is. Read its source if you need an API detail; the
  project may have customized it, and its version wins over the template.
- **Not found** and the task needs it → ask the user before installing (it adds
  ~400 lines to their repo), then run:

  ```bash
  bash <skill-dir>/scripts/install.sh <project-root>
  ```

  It copies `bus`, `cqrs`, `events` and `ddd` into
  `<project>/internal/platform/`, rewrites imports to the project module,
  switches to `github.com/google/uuid` on Go 1.24–1.26 (Go 1.27+ has stdlib
  `uuid`), refuses Go < 1.24 or an existing install, and runs `go vet` +
  `go test`. If the project uses google/uuid, write `uuid.Must(uuid.NewV7())`
  and `uuid.Nil` wherever the examples show `uuid.NewV7()` and `uuid.Nil()`.

## 2. Platform API at a glance

| Package | Use |
|---|---|
| `bus` | `bus.New()` sequential bus (default). `bus.NewConcurrent(timeout, limit)` only when profiling shows dispatch is the bottleneck; call `Wait()` on shutdown. |
| `cqrs` | `Command`/`Query` (`Name() string`, constant, value receiver). `CommandHandler[C]` returns `([]events.Event, error)`; `QueryHandler[Q, R]` returns `(R, error)`. `RegisterCommand`, `RegisterQuery`, `Send`, `Ask[R]`. `WrapCommand`/`WrapQuery` apply middleware, first = outermost. |
| `cqrs` middleware | `LogCommandErrors`, `LogQueryErrors`, `PublishEvents(eventBus)`, `WithUnitOfWork(uow)`; `ErrEventsNotPublished`. |
| `events` | Embed `events.Base` (`events.NewBase(name, aggregateID)`) in concrete events. `events.Register(bus, name, handlers...)`, `events.Publish`, `events.Listen` (channel → bus bridge). |
| `ddd` | Embed `ddd.AggregateRoot`; `RecordEvent` in domain methods, `PullEvents` in the command handler. |

Full source: `<skill-dir>/assets/template/internal/platform/`; wiring example
in `cqrs/example_test.go`.

## 3. Layout

Follows the project-layout conventions (`cmd/` thin, code in `internal/`), one
directory per bounded context:

```
cmd/<service>/main.go          composition root: buses, middleware, registration
internal/
  platform/{bus,cqrs,events,ddd}
  <context>/                   e.g. ordering, shipping
    domain/                    aggregates, value objects, events, repository ports
    app/                       commands, queries, handlers, policies, read models
    infra/<tech>/              repositories, unit of work, HTTP/gRPC adapters
```

Dependencies point inward: `infra → app → domain → platform`. `domain` never
imports `app` or `infra`; `app` never imports `infra`. If the project already
has its own layout, fit into it rather than imposing this one.

## 4. Rules, and why

1. **Commands change state; queries don't.** A command handler returns only
   events and an error, never read data. A query handler never writes and never
   raises events. Mixing them breaks the ability to scale, cache and reason
   about each side separately. If a caller needs data after a command, it
   generates the ID up front (`uuid.NewV7()`) and then asks a query.
2. **Naming carries meaning.** Commands are imperative (`PlaceOrder`, name
   `ordering.place_order`); events are past tense (`OrderPlaced`, name
   `ordering.order_placed`); queries describe what they get (`GetOrder`).
   Prefix names with the bounded context so they stay unique on shared buses.
   Keep event names as exported constants next to the event type.
3. **One command, one aggregate, one transaction.** The handler loads or
   creates one aggregate, calls a domain method, saves it and returns
   `PullEvents()`. Effects on other aggregates happen through events → policies
   → commands, accepting eventual consistency. This keeps transactions small
   and contexts decoupled.
4. **Business rules live in the domain.** Handlers orchestrate (load, call,
   save); aggregates enforce invariants and record events. Domain errors are
   sentinel/typed errors in `domain` so adapters can map them.
5. **Policies only translate.** An event handler maps an event to a command and
   `cqrs.Send`s it. Command handlers never call other command handlers directly.
   Event handlers must be idempotent: the same event may arrive twice.
6. **Middleware order:** `LogCommandErrors → PublishEvents → WithUnitOfWork →
   handler`. The unit of work is innermost so events are published only after
   commit; logging is outermost so it sees every failure. Build the slice once
   in `main` and reuse it for every command.
7. **Adapters stay dumb.** HTTP/gRPC/consumer code decodes input, calls
   `cqrs.Send`/`cqrs.Ask`, maps errors to protocol codes, encodes output.
   Treat `errors.Is(err, cqrs.ErrEventsNotPublished)` as success-with-warning:
   the state did change.
8. **Know the limits of in-process events.** `PublishEvents` runs after commit
   but is not atomic with it: a crash in between loses the events. When events
   must not be lost or must reach other services, use the transactional outbox
   pattern and a broker library (e.g. Watermill), feeding consumers into the
   bus with `events.Listen`. Don't grow the platform into a broker client.
9. **Queries return read models**, not aggregates — DTOs shaped for the caller,
   loaded through a read-side port that may query the DB directly.
10. **Test handlers with hand-written fakes** of the ports, table-driven, and
    assert on returned events and errors. Test aggregates directly.

## 5. Workflows

Read `references/patterns.md` before writing code in any of these; it contains a
compiling example of every piece (domain, command, query, policy, unit of work,
repository, HTTP adapter, `main.go`, tests).

**Add a command (write use case)**
1. Domain: add/extend the aggregate method that enforces the rule and calls
   `RecordEvent`; define the event type and its name constant; extend the
   repository port if needed.
2. App: command struct + `Name()`, handler struct with port dependencies.
3. Infra: implement new port methods; use the transaction from the context.
4. `main.go`: `cqrs.RegisterCommand(commands, cqrs.WrapCommand(h, commandMws...))`.
5. Adapter route, if any. Tests for the aggregate and the handler.

**Add a query (read use case)**
1. App: query struct + `Name()`, view DTO, read port, handler.
2. Infra: implement the read port. 3. `main.go`: `cqrs.RegisterQuery(...)`.
4. Adapter route, if any. Test the handler.

**React to an event**
1. In the reacting context's `app`: the command to run and a policy
   implementing `events.Handler[E]` that sends it.
2. `main.go`: `events.Register(eventBus, EventName, policies...)`. Register
   each event name once, passing all its handlers in that call.

**New service**: install the platform, create `cmd/<service>/main.go` following
section 7 of `references/patterns.md`, then add use cases as above.

## 6. Reviewing code

When asked to review, check the rules in section 4 and report each violation
with file:line, the rule broken and a concrete fix. Common findings: query
handlers that write; command handlers returning data or touching two
aggregates; business rules in handlers or adapters; handlers calling handlers;
`infra` types imported by `domain`/`app`; middleware with the unit of work
outside `PublishEvents`; event handlers with side effects that aren't
idempotent; events published before commit.
