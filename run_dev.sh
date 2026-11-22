#!/bin/sh
#
# Development runner. Uses Air for hot reload when it is installed, otherwise
# falls back to `go run`.
#
set -eu

if command -v air >/dev/null 2>&1; then
    echo "==> starting with air (hot reload)"
    exec air
fi

echo "==> air is not installed, running 'go run .'"
echo "    install it with: go install github.com/air-verse/air@latest"
exec go run .