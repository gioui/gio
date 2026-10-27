#!/bin/sh
# SPDX-License-Identifier: Unlicense OR MIT
#
# Guard the OHOS build-tag contract:
#   * every file that positively selects OHOS must use both aliases,
#     unless it is the host-only test tag;
#   * every file that excludes one alias must exclude the other.
#
# This keeps "gio_ohos" canonical while "openharmony" remains a temporary
# compatibility alias.
set -eu

cd "$(dirname "$0")/.."
fail=0

for f in $(find app internal gpu -name '*.go' -type f | sort); do
	tag=$(sed -n 's|^//go:build ||p' "$f" | head -n 1)
	[ -n "$tag" ] || continue

	# Negative clauses must come in pairs.
	case "$tag" in
	*'!gio_ohos'*)
		case "$tag" in
		*'!openharmony'*) ;;
		*) echo "missing !openharmony in $f: $tag"; fail=1 ;;
		esac
		;;
	esac
	case "$tag" in
	*'!openharmony'*)
		case "$tag" in
		*'!gio_ohos'*) ;;
		*) echo "missing !gio_ohos in $f: $tag"; fail=1 ;;
		esac
		;;
	esac

	# Positive clauses: host-only tests are exempt.
	pos=$(printf '%s\n' "$tag" | sed 's/![A-Za-z0-9_]*//g')
	case "$pos" in
	*gio_ohos*|*openharmony*)
		case "$tag" in
		*gio_ohos_test*) ;;
		*)
			case "$pos" in
			*gio_ohos*) ;;
			*) echo "missing gio_ohos in $f: $tag"; fail=1 ;;
			esac
			case "$pos" in
			*openharmony*) ;;
			*) echo "missing openharmony in $f: $tag"; fail=1 ;;
			esac
			;;
		esac
		;;
	esac
done

if [ "$fail" -ne 0 ]; then
	echo "OHOS build tags are inconsistent" >&2
	exit 1
fi
echo "OHOS build tags OK"
