#!/usr/bin/env bash
# Regenerates skills/cqrs-eda/references/patterns.md from dev/sample-shop,
# after checking that the sample compiles and its tests pass.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$root/dev/sample-shop"
out="$root/skills/cqrs-eda/references/patterns.md"

(cd "$src" && test -z "$(gofmt -l .)" && go vet ./... && go test ./... >/dev/null)

sec() { printf '\n## %s\n\n%s\n\n`%s`\n\n```go\n' "$1" "$2" "$3"; cat "$src/$3"; printf '```\n'; }
{
cat <<'HDR'
# CQRS/EDA patterns — worked example

A complete, compiling slice of a service (module `github.com/acme/shop`, Go 1.27)
built on the platform packages. Copy the shape, not the names. Every snippet was
verified with `go vet` and `go test`. On Go < 1.27 the import is
`github.com/google/uuid`; write `uuid.Must(uuid.NewV7())` instead of `uuid.NewV7()`.

## Contents

1. Domain: aggregate, event, errors, repository port
2. Command and handler
3. Query, read model and handler
4. Policy: event → command (another bounded context)
5. Unit of work and repository (database/sql)
6. HTTP driving adapter
7. Composition root (`cmd/<service>/main.go`)
8. Testing a command handler
HDR
sec "1. Domain: aggregate, event, errors, repository port" "Pure Go: no SQL, HTTP or bus imports. Invariants and domain errors live here." internal/ordering/domain/order.go
sec "2. Command and handler" "One command, one handler, one aggregate. The handler orchestrates; the aggregate decides." internal/ordering/app/place_order.go
sec "3. Query, read model and handler" "Queries return views shaped for the caller and may bypass the aggregate." internal/ordering/app/get_order.go
sec "4. Policy: event → command" "Lives in the reacting context. Cross-aggregate effects go through events, so they are eventually consistent. Policies may receive the same event twice: the command they send must be idempotent." internal/shipping/app/policy.go
sec "5a. Unit of work (database/sql)" "Puts the transaction in the context; repositories pick it up via \`conn\`. Adapt to pgx by swapping the types." internal/ordering/infra/postgres/uow.go
sec "5b. Repository" "Implements the write port (domain) and the read port (app). Translates storage errors into domain errors." internal/ordering/infra/postgres/orders.go
sec "6. HTTP driving adapter" "Decode → \`cqrs.Send\`/\`cqrs.Ask\` → map domain errors to status codes → encode." internal/ordering/infra/httpapi/handlers.go
sec "7. Composition root" "The only place that knows every layer. Register the SQL driver (e.g. \`_ \"github.com/jackc/pgx/v5/stdlib\"\`) in real code." cmd/shop/main.go
sec "8. Testing a command handler" "Table-driven, hand-written fakes for ports; assert returned events, not internals." internal/ordering/app/place_order_test.go
} > "$out"
echo "wrote $out"
