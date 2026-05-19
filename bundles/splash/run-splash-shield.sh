#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"

mkdir -p .shield/transient/cmd
if [ ! -f .shield/transient/cmd/go.mod ]; then
	printf 'module shieldtransient\n\ngo 1.25\n' > .shield/transient/cmd/go.mod
fi

if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
	git init >/dev/null
fi

exec ./bin/shield-splash -configuration-path=shield_config.toml "$@"
