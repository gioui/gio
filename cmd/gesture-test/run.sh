#!/bin/sh
# Run the gesture diagnostic app with a consistent Go toolchain.
# The shell may resolve "go" to an older OHOS Go binary while GOROOT points
# at Homebrew Go 1.27; that combination fails with "version does not match".
set -eu
cd "$(dirname "$0")/../.."

GO_BIN="${GO:-}"
if [ -z "$GO_BIN" ]; then
	for c in \
		/opt/homebrew/Cellar/go/1.27.1/libexec/bin/go \
		/opt/homebrew/opt/go/libexec/bin/go
	do
		if [ -x "$c" ]; then
			GO_BIN="$c"
			break
		fi
	done
fi
if [ -z "$GO_BIN" ]; then
	GO_BIN="$(command -v go)"
fi
case "$GO_BIN" in
	/*) ;;
	*) GO_BIN="$(command -v "$GO_BIN")" ;;
esac

GOROOT="$(cd "$(dirname "$GO_BIN")/.." && pwd)"
echo "using $GO_BIN (GOROOT=$GOROOT)" >&2
exec env GOROOT="$GOROOT" "$GO_BIN" run ./cmd/gesture-test "$@"
