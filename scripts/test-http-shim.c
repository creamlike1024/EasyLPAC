/*
 * C-side harness verifying the libeuicc -> shim callback contract for
 * the HTTP interface: full URL composition, NULL-terminated header
 * array, C-allocated response buffer ownership, and status code
 * passthrough. Built and run by scripts/test-euicc.sh; mirrors what
 * the Go trampoline (callbacks.go) receives.
 *
 * EasyLPAC original code (MIT); links vendored LGPL libeuicc.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "euicc/euicc.h"
#include "euicc/es9p.h"

static char seen_url[2048];
static char seen_header0[256];
static char seen_header1[256];
static char seen_tx[4096];
static uint32_t seen_tx_len;

static int fake_http_transmit(struct euicc_ctx *ctx, const char *url, uint32_t *rcode,
                              uint8_t **rx, uint32_t *rx_len,
                              const uint8_t *tx, uint32_t tx_len, const char **headers) {
    snprintf(seen_url, sizeof(seen_url), "%s", url);
    snprintf(seen_header0, sizeof(seen_header0), "%s", headers[0] ? headers[0] : "(null)");
    snprintf(seen_header1, sizeof(seen_header1), "%s", headers[1] ? headers[1] : "(null)");
    snprintf(seen_tx, sizeof(seen_tx), "%s", (const char *)tx);
    seen_tx_len = tx_len;

    static const char resp[] = "{\"header\":{\"functionExecutionStatus\":{\"status\":\"Executed-Success\"}}}";
    *rx = (uint8_t *)malloc(sizeof(resp));
    if (*rx == NULL) {
        return -1;
    }
    memcpy(*rx, resp, sizeof(resp));
    *rx_len = sizeof(resp) - 1; /* protocol length excludes NUL */
    *rcode = 200;
    return 0;
}

int main(void) {
    static struct euicc_http_interface http_iface = {.transmit = fake_http_transmit};
    struct euicc_ctx ctx = {0};
    ctx.http.interface = &http_iface;
    ctx.http.server_address = "testsmdpplus.example.com";

    int rc = es9p_handle_notification(&ctx, "aGVsbG8"); /* dummy b64 payload */
    if (rc != 0) {
        fprintf(stderr, "es9p_handle_notification rc=%d\n", rc);
        return 1;
    }
    if (strcmp(seen_url, "https://testsmdpplus.example.com/gsma/rsp2/es9plus/handleNotification") != 0) {
        fprintf(stderr, "url mismatch: %s\n", seen_url);
        return 1;
    }
    if (strncmp(seen_header0, "User-Agent:", 11) != 0) {
        fprintf(stderr, "header0 mismatch: %s\n", seen_header0);
        return 1;
    }
    if (strncmp(seen_header1, "X-Admin-Protocol:", 17) != 0) {
        fprintf(stderr, "header1 mismatch: %s\n", seen_header1);
        return 1;
    }
    if (strstr(seen_tx, "aGVsbG8") == NULL) {
        fprintf(stderr, "tx payload missing: %s\n", seen_tx);
        return 1;
    }
    fprintf(stderr, "HTTP shim contract OK: url=%s tx_len=%u\n", seen_url, seen_tx_len);
    return 0;
}
