/*
 * Original C shim implementation. See shim.h for the license and
 * contract notes. Only this file touches libeuicc internals
 * (struct euicc_ctx fields); upgrade-sensitive accesses are marked
 * UPGRADE-SENSITIVE and must be re-checked when bumping libeuicc.
 */
#include "shim.h"

#include <stdlib.h>
#include <string.h>

#include "euicc/euicc.h"
#include "euicc/es10a.h"
#include "euicc/es10b.h"
#include "euicc/es10c.h"
#include "euicc/es10c_ex.h"
#include "euicc/es8p.h"
#include "euicc/es9p.h"
#include "euicc/interface.h"

/* Go trampoline entry points (exported via //export in callbacks.go). */
extern int elpacGoApduConnect(uintptr_t token);
extern void elpacGoApduDisconnect(uintptr_t token);
extern int elpacGoApduLogicChannelOpen(uintptr_t token, const uint8_t *aid, uint8_t aid_len);
extern void elpacGoApduLogicChannelClose(uintptr_t token, uint8_t channel);
extern int elpacGoApduTransmit(uintptr_t token, const uint8_t *tx, uint32_t tx_len,
                               uint8_t **rx, uint32_t *rx_len);
extern int elpacGoHttpTransmit(uintptr_t token, const char *url, const char **headers,
                               const uint8_t *tx, uint32_t tx_len,
                               uint8_t **rx, uint32_t *rx_len, uint32_t *rcode);

struct elpac_ctx {
    struct euicc_ctx ctx;
    struct euicc_apdu_interface apdu_iface;
    struct euicc_http_interface http_iface;
    uintptr_t apdu_token;
    uintptr_t http_token;
    /* Owned copy of the SM-DP+ address handed to ctx.http. */
    char *server_address_copy;
};

static int shim_connect(struct euicc_ctx *ctx) {
    struct elpac_ctx *c = (struct elpac_ctx *)ctx;
    return elpacGoApduConnect(c->apdu_token);
}

static void shim_disconnect(struct euicc_ctx *ctx) {
    struct elpac_ctx *c = (struct elpac_ctx *)ctx;
    elpacGoApduDisconnect(c->apdu_token);
}

static int shim_logic_channel_open(struct euicc_ctx *ctx, const uint8_t *aid, uint8_t aid_len) {
    struct elpac_ctx *c = (struct elpac_ctx *)ctx;
    return elpacGoApduLogicChannelOpen(c->apdu_token, aid, aid_len);
}

static void shim_logic_channel_close(struct euicc_ctx *ctx, uint8_t channel) {
    struct elpac_ctx *c = (struct elpac_ctx *)ctx;
    elpacGoApduLogicChannelClose(c->apdu_token, channel);
}

static int shim_transmit(struct euicc_ctx *ctx, uint8_t **rx, uint32_t *rx_len,
                         const uint8_t *tx, uint32_t tx_len) {
    struct elpac_ctx *c = (struct elpac_ctx *)ctx;
    return elpacGoApduTransmit(c->apdu_token, tx, tx_len, rx, rx_len);
}

static int shim_http_transmit(struct euicc_ctx *ctx, const char *url, uint32_t *rcode,
                              uint8_t **rx, uint32_t *rx_len,
                              const uint8_t *tx, uint32_t tx_len, const char **headers) {
    struct elpac_ctx *c = (struct elpac_ctx *)ctx;
    return elpacGoHttpTransmit(c->http_token, url, headers, tx, tx_len, rx, rx_len, rcode);
}

elpac_ctx *elpac_new(const uint8_t *aid, uint8_t aid_len) {
    struct elpac_ctx *c = calloc(1, sizeof(*c));
    if (c == NULL) {
        return NULL;
    }
    uint8_t *aid_copy = malloc(aid_len);
    if (aid_copy == NULL) {
        free(c);
        return NULL;
    }
    memcpy(aid_copy, aid, aid_len);
    /* UPGRADE-SENSITIVE: ctx.aid / aid_len fields. */
    c->ctx.aid = aid_copy;
    c->ctx.aid_len = aid_len;
    return c;
}

int elpac_set_interfaces(elpac_ctx *c, uintptr_t apdu_token, uintptr_t http_token) {
    c->apdu_token = apdu_token;
    c->http_token = http_token;

    c->apdu_iface.connect = shim_connect;
    c->apdu_iface.disconnect = shim_disconnect;
    c->apdu_iface.logic_channel_open = shim_logic_channel_open;
    c->apdu_iface.logic_channel_close = shim_logic_channel_close;
    c->apdu_iface.transmit = shim_transmit;
    /* UPGRADE-SENSITIVE: ctx.apdu.interface pointer. */
    c->ctx.apdu.interface = &c->apdu_iface;

    c->http_iface.transmit = shim_http_transmit;
    /* UPGRADE-SENSITIVE: ctx.http.interface pointer. */
    c->ctx.http.interface = &c->http_iface;
    return 0;
}

int elpac_init(elpac_ctx *c) {
    return euicc_init(&c->ctx);
}

void elpac_fini(elpac_ctx *c) {
    euicc_fini(&c->ctx);
}

void elpac_http_cleanup(elpac_ctx *c) {
    euicc_http_cleanup(&c->ctx);
}

void elpac_free(elpac_ctx *c) {
    if (c == NULL) {
        return;
    }
    /* Make callbacks unreachable before the Go side deletes handles. */
    c->ctx.apdu.interface = NULL;
    c->ctx.http.interface = NULL;
    c->ctx.http.server_address = NULL;
    free(c->server_address_copy);
    free((void *)c->ctx.aid);
    free(c);
}

void elpac_set_debug(elpac_ctx *c, int apdu_debug, int http_debug) {
    /* UPGRADE-SENSITIVE: ctx.apdu.log_fp / ctx.http.log_fp fields. */
    c->ctx.apdu.log_fp = apdu_debug ? stderr : NULL;
    c->ctx.http.log_fp = http_debug ? stderr : NULL;
}

int elpac_get_eid(elpac_ctx *c, char **eid_out) {
    return es10c_get_eid(&c->ctx, eid_out);
}

int elpac_get_configured_addresses(elpac_ctx *c, char **default_dp_out, char **root_ds_out) {
    struct es10a_euicc_configured_addresses addr = {0};
    int rc = es10a_get_euicc_configured_addresses(&c->ctx, &addr);
    if (rc != 0) {
        return rc;
    }
    *default_dp_out = addr.defaultDpAddress; /* ownership transferred */
    *root_ds_out = addr.rootDsAddress;
    return 0;
}

/* strdup helper; NULL-safe. */
static char *dup_or_null(const char *s) {
    return s ? strdup(s) : NULL;
}

/* Copy a NULL-terminated string array into a newly allocated
 * NULL-terminated array of strdup'd strings. NULL input yields NULL. */
static const char **dup_str_array(const char **src) {
    if (!src) {
        return NULL;
    }
    size_t n = 0;
    while (src[n]) {
        n++;
    }
    const char **out = calloc(n + 1, sizeof(char *));
    if (!out) {
        return NULL;
    }
    for (size_t i = 0; i < n; i++) {
        out[i] = dup_or_null(src[i]);
        if (!out[i]) {
            for (size_t j = 0; j < i; j++) {
                free((void *)out[j]);
            }
            free(out);
            return NULL;
        }
    }
    return out;
}

static void free_str_array(const char **arr) {
    if (!arr) {
        return;
    }
    for (size_t i = 0; arr[i]; i++) {
        free((void *)arr[i]);
    }
    free((void *)arr);
}

int elpac_get_euiccinfo2(elpac_ctx *c, struct elpac_euiccinfo2 *out) {
    struct es10c_ex_euiccinfo2 info = {0};
    memset(out, 0, sizeof(*out));

    if (es10c_ex_get_euiccinfo2(&c->ctx, &info) != 0) {
        return -1;
    }

    out->profileVersion = dup_or_null(info.profileVersion);
    out->svn = dup_or_null(info.svn);
    out->euiccFirmwareVer = dup_or_null(info.euiccFirmwareVer);
    out->installedApplication = info.extCardResource.installedApplication;
    out->freeNonVolatileMemory = info.extCardResource.freeNonVolatileMemory;
    out->freeVolatileMemory = info.extCardResource.freeVolatileMemory;
    out->uiccCapability = dup_str_array(info.uiccCapability);
    out->ts102241Version = dup_or_null(info.ts102241Version);
    out->globalplatformVersion = dup_or_null(info.globalplatformVersion);
    out->rspCapability = dup_str_array(info.rspCapability);
    out->euiccCiPKIdListForVerification = dup_str_array((const char **)info.euiccCiPKIdListForVerification);
    out->euiccCiPKIdListForSigning = dup_str_array((const char **)info.euiccCiPKIdListForSigning);
    out->euiccCategory = dup_or_null(info.euiccCategory);
    out->forbiddenProfilePolicyRules = dup_str_array(info.forbiddenProfilePolicyRules);
    out->ppVersion = dup_or_null(info.ppVersion);
    out->sasAcreditationNumber = dup_or_null(info.sasAcreditationNumber);
    out->platformLabel = dup_or_null(info.certificationDataObject.platformLabel);
    out->discoveryBaseURL = dup_or_null(info.certificationDataObject.discoveryBaseURL);

    es10c_ex_euiccinfo2_free(&info);
    return 0;
}

void elpac_euiccinfo2_free(struct elpac_euiccinfo2 *info) {
    if (!info) {
        return;
    }
    free(info->profileVersion);
    free(info->svn);
    free(info->euiccFirmwareVer);
    free_str_array(info->uiccCapability);
    free(info->ts102241Version);
    free(info->globalplatformVersion);
    free_str_array(info->rspCapability);
    free_str_array(info->euiccCiPKIdListForVerification);
    free_str_array(info->euiccCiPKIdListForSigning);
    free(info->euiccCategory);
    free_str_array(info->forbiddenProfilePolicyRules);
    free(info->ppVersion);
    free(info->sasAcreditationNumber);
    free(info->platformLabel);
    free(info->discoveryBaseURL);
    memset(info, 0, sizeof(*info));
}

int elpac_get_profiles(elpac_ctx *c, struct elpac_profile_list *out) {
    struct es10c_profile_info_list *list = NULL;
    memset(out, 0, sizeof(*out));

    if (es10c_get_profiles_info(&c->ctx, &list) != 0) {
        return -1;
    }

    uint32_t count = 0;
    for (struct es10c_profile_info_list *p = list; p; p = p->next) {
        count++;
    }
    if (count == 0) {
        es10c_profile_info_list_free_all(list);
        return 0;
    }

    struct elpac_profile *items = calloc(count, sizeof(*items));
    if (!items) {
        es10c_profile_info_list_free_all(list);
        return -1;
    }

    uint32_t i = 0;
    for (struct es10c_profile_info_list *p = list; p; p = p->next, i++) {
        items[i].iccid = strdup(p->iccid);
        items[i].isdpAid = strdup(p->isdpAid);
        items[i].profileState = (int32_t)p->profileState;
        items[i].profileClass = (int32_t)p->profileClass;
        items[i].profileNickname = dup_or_null(p->profileNickname);
        items[i].serviceProviderName = dup_or_null(p->serviceProviderName);
        items[i].profileName = dup_or_null(p->profileName);
        items[i].iconType = (int32_t)p->iconType;
        items[i].iconBase64 = dup_or_null(p->icon);
    }
    es10c_profile_info_list_free_all(list);

    out->items = items;
    out->count = count;
    return 0;
}

void elpac_profile_list_free(struct elpac_profile_list *list) {
    if (!list || !list->items) {
        return;
    }
    for (uint32_t i = 0; i < list->count; i++) {
        free((void *)list->items[i].iccid);
        free((void *)list->items[i].isdpAid);
        free((void *)list->items[i].profileNickname);
        free((void *)list->items[i].serviceProviderName);
        free((void *)list->items[i].profileName);
        free((void *)list->items[i].iconBase64);
    }
    free(list->items);
    list->items = NULL;
    list->count = 0;
}

int elpac_get_notifications(elpac_ctx *c, struct elpac_notification_list *out) {
    struct es10b_notification_metadata_list *list = NULL;
    memset(out, 0, sizeof(*out));

    if (es10b_list_notification(&c->ctx, &list) != 0) {
        return -1;
    }

    uint32_t count = 0;
    for (struct es10b_notification_metadata_list *p = list; p; p = p->next) {
        count++;
    }
    if (count == 0) {
        es10b_notification_metadata_list_free_all(list);
        return 0;
    }

    struct elpac_notification *items = calloc(count, sizeof(*items));
    if (!items) {
        es10b_notification_metadata_list_free_all(list);
        return -1;
    }

    uint32_t i = 0;
    for (struct es10b_notification_metadata_list *p = list; p; p = p->next, i++) {
        items[i].seqNumber = p->seqNumber;
        items[i].profileManagementOperation = (int32_t)p->profileManagementOperation;
        items[i].notificationAddress = dup_or_null(p->notificationAddress);
        items[i].iccid = dup_or_null(p->iccid);
    }
    es10b_notification_metadata_list_free_all(list);

    out->items = items;
    out->count = count;
    return 0;
}

void elpac_notification_list_free(struct elpac_notification_list *list) {
    if (!list || !list->items) {
        return;
    }
    for (uint32_t i = 0; i < list->count; i++) {
        free((void *)list->items[i].notificationAddress);
        free((void *)list->items[i].iccid);
    }
    free(list->items);
    list->items = NULL;
    list->count = 0;
}

void elpac_string_free(char *s) {
    free(s);
}

/* --- Write operations (P5) --------------------------------------- */

int elpac_profile_enable(elpac_ctx *c, const char *iccid, int refresh) {
    return es10c_enable_profile(&c->ctx, iccid, refresh ? 0xFF : 0);
}

int elpac_profile_disable(elpac_ctx *c, const char *iccid, int refresh) {
    return es10c_disable_profile(&c->ctx, iccid, refresh ? 0xFF : 0);
}

int elpac_profile_delete(elpac_ctx *c, const char *iccid) {
    return es10c_delete_profile(&c->ctx, iccid);
}

int elpac_set_nickname(elpac_ctx *c, const char *iccid, const char *nickname) {
    return es10c_set_nickname(&c->ctx, iccid, nickname);
}

int elpac_set_default_dp(elpac_ctx *c, const char *smdp) {
    return es10a_set_default_dp_address(&c->ctx, smdp);
}

int elpac_notification_retrieve(elpac_ctx *c, uint64_t seq, char **address_out, char **b64_out) {
    struct es10b_pending_notification pending = {0};
    *address_out = NULL;
    *b64_out = NULL;

    if (es10b_retrieve_notifications_list(&c->ctx, &pending, (unsigned long)seq) != 0) {
        return -1;
    }
    *address_out = dup_or_null(pending.notificationAddress);
    *b64_out = dup_or_null(pending.b64_PendingNotification);
    int rc = (*address_out && *b64_out) ? 0 : -1;
    es10b_pending_notification_free(&pending);
    return rc;
}

int elpac_notification_send(elpac_ctx *c, const char *b64_pending) {
    return es9p_handle_notification(&c->ctx, b64_pending);
}

int elpac_notification_remove(elpac_ctx *c, uint64_t seq) {
    return es10b_remove_notification_from_list(&c->ctx, (unsigned long)seq);
}

/* --- Download orchestration (P5) --------------------------------- */

/* UPGRADE-SENSITIVE: ctx.http.server_address ownership. libeuicc reads
 * it during ES9+ calls only; the copy is freed in elpac_free. */
int elpac_set_server_address(elpac_ctx *c, const char *smdp) {
    char *copy = dup_or_null(smdp);
    if (!copy) {
        return -1;
    }
    free((void *)c->server_address_copy);
    c->server_address_copy = copy;
    c->ctx.http.server_address = copy;
    return 0;
}

int elpac_download_get_challenge_and_info(elpac_ctx *c) {
    return es10b_get_euicc_challenge_and_info(&c->ctx);
}

int elpac_download_initiate_authentication(elpac_ctx *c) {
    return es9p_initiate_authentication(&c->ctx);
}

int elpac_download_authenticate_server(elpac_ctx *c, const char *matching_id, const char *imei) {
    return es10b_authenticate_server(&c->ctx, matching_id, imei);
}

int elpac_download_authenticate_client(elpac_ctx *c) {
    return es9p_authenticate_client(&c->ctx);
}

int elpac_download_prepare_download(elpac_ctx *c, const char *confirmation_code) {
    return es10b_prepare_download(&c->ctx, confirmation_code);
}

int elpac_download_get_bound_profile_package(elpac_ctx *c) {
    return es9p_get_bound_profile_package(&c->ctx);
}

int elpac_download_load_bound_profile_package(elpac_ctx *c, struct elpac_download_result *out) {
    struct es10b_load_bound_profile_package_result result = {0};
    memset(out, 0, sizeof(*out));

    if (es10b_load_bound_profile_package(&c->ctx, &result) != 0) {
        /* Propagate the failure detail for diagnostics. */
        out->seq_number = result.seqNumber;
        out->bpp_command_id = (int32_t)result.bppCommandId;
        out->error_reason = (int32_t)result.errorReason;
        out->iccid = dup_or_null(result.iccid);
        free(result.iccid);
        return -1;
    }
    out->seq_number = result.seqNumber;
    out->bpp_command_id = (int32_t)result.bppCommandId;
    out->error_reason = (int32_t)result.errorReason;
    out->iccid = dup_or_null(result.iccid);
    free(result.iccid);
    return 0;
}

void elpac_download_result_free(struct elpac_download_result *result) {
    if (!result) {
        return;
    }
    free(result->iccid);
    memset(result, 0, sizeof(*result));
}

int elpac_download_cancel(elpac_ctx *c, int reason) {
    int rc = es10b_cancel_session(&c->ctx, (enum es10b_cancel_session_reason)reason);
    int rc2 = es9p_cancel_session(&c->ctx);
    return (rc == 0 && rc2 == 0) ? 0 : -1;
}

/* UPGRADE-SENSITIVE: ctx.http._internal.prepare_download_param. */
int elpac_download_metadata(elpac_ctx *c, struct elpac_profile_metadata *out) {
    struct es8p_metadata *meta = NULL;
    memset(out, 0, sizeof(*out));

    const char *b64 = NULL;
    if (c->ctx.http._internal.prepare_download_param) {
        b64 = c->ctx.http._internal.prepare_download_param->b64_profileMetadata;
    }
    if (!b64) {
        return -1;
    }
    if (es8p_metadata_parse(&meta, b64) != 0 || !meta) {
        return -1;
    }

    out->iccid = dup_or_null(meta->iccid);
    out->serviceProviderName = dup_or_null(meta->serviceProviderName);
    out->profileName = dup_or_null(meta->profileName);
    out->iconType = (int32_t)meta->iconType;
    out->iconBase64 = dup_or_null(meta->icon);
    out->profileClass = (int32_t)meta->profileClass;

    es8p_metadata_free(&meta);
    return 0;
}

void elpac_profile_metadata_free(struct elpac_profile_metadata *meta) {
    if (!meta) {
        return;
    }
    free(meta->iccid);
    free(meta->serviceProviderName);
    free(meta->profileName);
    free(meta->iconBase64);
    memset(meta, 0, sizeof(*meta));
}

/* UPGRADE-SENSITIVE: ctx.http.status fields. */
const char *elpac_http_status_subject_code(elpac_ctx *c) {
    return c->ctx.http.status.subjectCode;
}

const char *elpac_http_status_reason_code(elpac_ctx *c) {
    return c->ctx.http.status.reasonCode;
}

const char *elpac_http_status_message(elpac_ctx *c) {
    return c->ctx.http.status.message;
}

int elpac_http_status_is_error(elpac_ctx *c) {
    const char *subject = c->ctx.http.status.subjectCode;
    const char *reason = c->ctx.http.status.reasonCode;
    return !(strcmp(subject, "0.0.0") == 0 && strcmp(reason, "0.0.0") == 0);
}

