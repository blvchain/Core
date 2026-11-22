#!/bin/sh
#
# Build the node binary.
#
# Plain `go build` is the default so the script works on any machine with a Go
# toolchain. Set USE_GARBLE=1 / USE_UPX=1 to enable the optional hardening and
# compression steps (both tools must be installed).
#
#   ./build.sh                     # ./core
#   OUTPUT=node1 ./build.sh        # ./node1
#   USE_GARBLE=1 ./build.sh        # obfuscated build
#
set -eu

OUTPUT=${OUTPUT:-core}
USE_GARBLE=${USE_GARBLE:-0}
USE_UPX=${USE_UPX:-0}

echo "==> building $OUTPUT"

if [ "$USE_GARBLE" = "1" ]; then
    if ! command -v garble >/dev/null 2>&1; then
        echo "error: USE_GARBLE=1 but garble is not installed" >&2
        exit 1
    fi
    garble -literals -tiny build -trimpath -ldflags="-s -w -buildid=" -o "$OUTPUT" main.go
else
    go build -trimpath -ldflags="-s -w -buildid=" -o "$OUTPUT" .
fi

echo "==> built $OUTPUT"

if [ "$USE_UPX" = "1" ]; then
    if ! command -v upx >/dev/null 2>&1; then
        echo "error: USE_UPX=1 but upx is not installed" >&2
        exit 1
    fi
    upx --best --lzma "$OUTPUT"
    echo "==> compressed $OUTPUT"
fi

# Deploy the binary to a directory when asked to:
#   TARGET_DIR=/path/to/node ./build.sh
if [ -n "${TARGET_DIR:-}" ]; then
    mkdir -p "$TARGET_DIR"
    cp "$OUTPUT" "$TARGET_DIR/"
    echo "==> copied $OUTPUT to $TARGET_DIR"
fi