#!/bin/sh
# SPDX-License-Identifier: Unlicense OR MIT
#
# Host-side C contract test for the OHOS ABI. Requires a host C compiler
# and (optionally) AddressSanitizer.
set -eu
cd "$(dirname "$0")/.."

CC=${CC:-cc}
OUT=${TMPDIR:-/tmp}/gio-ohos-abi-test
FLAGS="-std=c11 -Wall -Wextra -Werror -DGIO_OHOS_HOST_TEST -Iapp"

if "$CC" $FLAGS -fsanitize=address app/ohos_glue.c app/testdata/ohos_abi_test.c -o "$OUT" 2>/dev/null; then
	"$OUT"
else
	"$CC" $FLAGS app/ohos_glue.c app/testdata/ohos_abi_test.c -o "$OUT"
	"$OUT"
fi
