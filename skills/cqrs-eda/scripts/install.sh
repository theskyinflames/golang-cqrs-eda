#!/usr/bin/env bash
# Copies the CQRS/EDA platform packages into a Go project (copy-in model).
#
# Usage: install.sh [project-dir]   (default: current directory)
#
# - Copies assets/template/internal/platform/* to <project>/internal/platform/
# - Rewrites the placeholder module path to the project's module
# - Go 1.24–1.26 (no stdlib uuid): switches to github.com/google/uuid
# - Runs go vet and go test on the copied packages
set -euo pipefail

skill_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$skill_dir/assets/template/internal/platform"
placeholder="example.com/app"
pkgs=(bus cqrs events ddd)

project="$(cd "${1:-.}" && pwd)"
gomod="$project/go.mod"
dst="$project/internal/platform"

# --- checks: nothing is written until all pass ---
[[ -f "$gomod" ]] || { echo "error: no go.mod in $project" >&2; exit 1; }

module="$(awk '$1 == "module" { print $2; exit }' "$gomod")"
gover="$(awk '$1 == "go" { print $2; exit }' "$gomod")"
[[ -n "$module" ]] || { echo "error: no module directive in $gomod" >&2; exit 1; }
[[ -n "$gover" ]] || { echo "error: no go directive in $gomod" >&2; exit 1; }

IFS=. read -r major minor _ <<<"$gover"
if ((major == 1 && minor < 24)); then
	echo "error: go $gover is too old; the platform packages need go >= 1.24" >&2
	exit 1
fi

for pkg in "${pkgs[@]}"; do
	if [[ -e "$dst/$pkg" ]]; then
		echo "error: $dst/$pkg already exists; refusing to overwrite" >&2
		exit 1
	fi
done

# --- copy and rewrite ---
mkdir -p "$dst"
for pkg in "${pkgs[@]}"; do
	cp -R "$src/$pkg" "$dst/$pkg"
done
# No mapfile: macOS ships bash 3.2.
files=()
while IFS= read -r f; do files+=("$f"); done < <(for pkg in "${pkgs[@]}"; do find "$dst/$pkg" -name '*.go'; done)
sed -i.bak "s#\"$placeholder/internal/platform/#\"$module/internal/platform/#g" "${files[@]}"

if ((major == 1 && minor < 27)); then
	echo "go $gover has no stdlib uuid: using github.com/google/uuid"
	sed -i.bak \
		-e 's#^\([[:space:]]*\)"uuid"$#\1"github.com/google/uuid"#' \
		-e 's#uuid\.NewV7()#uuid.Must(uuid.NewV7())#g' \
		-e 's#uuid\.Nil()#uuid.Nil#g' \
		"${files[@]}"
fi
for f in "${files[@]}"; do rm -f "$f.bak"; done

# --- verify ---
cd "$project"
if ((major == 1 && minor < 27)); then
	go get github.com/google/uuid@latest
fi
gofmt -w "${files[@]}"
go vet ./internal/platform/...
go test ./internal/platform/...
echo "installed CQRS/EDA platform into $dst (module $module)"
