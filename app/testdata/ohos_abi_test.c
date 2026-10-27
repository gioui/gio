// SPDX-License-Identifier: Unlicense OR MIT
//
// Host-side contract test for the OHOS C ABI. Compile with
//   cc -DGIO_OHOS_HOST_TEST -Iapp app/ohos_glue.c app/testdata/ohos_abi_test.c
// See script/test-ohos-abi.sh.

#include <assert.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "ohos_glue.h"

static void test_v2_install(void) {
    gio_ohos_host *host = calloc(1, sizeof(*host));
    assert(host != NULL);
    host->user = (void *)0x1;
    gio_ohos_store_host(host);
    assert(gio_ohos_host_ready() == 1);
    assert(gio_ohos_host_abi_version() == GIO_OHOS_ABI2_VERSION);
    /* Missing callbacks must be tolerated. */
    gio_call_invalidate();
    gio_call_show_keyboard(1);
    free(host);
}

static void test_v3_install(void) {
    gio_ohos_host_v3 *host = calloc(1, sizeof(*host));
    assert(host != NULL);
    host->v2.user = (void *)0x2;
    gio_ohos_store_host_v3(host, sizeof(*host));
    assert(gio_ohos_host_ready() == 1);
    assert(gio_ohos_host_abi_version() == GIO_OHOS_ABI3_VERSION);
    assert(offsetof(gio_ohos_host_v3, set_window_mode) >= sizeof(gio_ohos_host));
    free(host);
}

static void test_v4_install(void) {
    gio_ohos_host_v4 *host = calloc(1, sizeof(*host));
    assert(host != NULL);
    host->v3.v2.user = (void *)0x3;
    gio_ohos_store_host_v4(host, sizeof(*host));
    assert(gio_ohos_host_ready() == 1);
    assert(gio_ohos_host_abi_version() == GIO_OHOS_ABI_VERSION);
    assert(offsetof(gio_ohos_host_v4, select_window) >= sizeof(gio_ohos_host_v3));
    free(host);
}

static void test_v3_size_through_v4(void) {
    gio_ohos_host_v3 *host = calloc(1, sizeof(*host));
    assert(host != NULL);
    gio_ohos_store_host_v4((const gio_ohos_host_v4 *)host, sizeof(*host));
    assert(gio_ohos_host_abi_version() == GIO_OHOS_ABI3_VERSION);
    free(host);
}

static void test_v2_size_through_v3(void) {
    /* A v2 shim calling the v3 entry with its own size must not be read
     * past the allocation (run this test under ASan). */
    gio_ohos_host *host = calloc(1, sizeof(*host));
    assert(host != NULL);
    gio_ohos_store_host_v3((const gio_ohos_host_v3 *)host, sizeof(*host));
    assert(gio_ohos_host_abi_version() == GIO_OHOS_ABI2_VERSION);
    free(host);
}

static void test_null(void) {
    gio_ohos_store_host_v3(NULL, 0);
    assert(gio_ohos_host_ready() == 0);
    assert(gio_ohos_host_abi_version() == 0);
    gio_ohos_store_host(NULL);
    assert(gio_ohos_host_ready() == 0);
}

int main(void) {
    test_v2_install();
    test_v3_install();
    test_v4_install();
    test_v3_size_through_v4();
    test_v2_size_through_v3();
    test_null();
    puts("OHOS ABI contract OK");
    return 0;
}
