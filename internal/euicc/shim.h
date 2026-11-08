/*
 * Original C shim between Go (cgo) and libeuicc.
 *
 * This file is EasyLPAC original code (MIT). It links against the
 * vendored libeuicc, which is LGPL-2.1-only OR LicenseRef-ESTKME-
 * Commercial (see third_party/lpac/euicc/LICENSE). No lpac CLI (AGPL)
 * code is used or referenced here.
 *
 * Contracts implemented here (docs/refactor-architecture.md §4):
 *  - opaque handle hides struct euicc_ctx;
 *  - C owns the context, the interface structs and the AID copy;
 *  - Go state is referenced via integer tokens (runtime/cgo.Handle);
 *  - APDU/HTTP response buffers are malloc'd; libeuicc frees them;
 *  - elpac_free is safe on any partially-initialized context and makes
 *    callbacks unreachable before Go handles are deleted.
 */
#ifndef ELPAC_SHIM_H
#define ELPAC_SHIM_H

#include <stdint.h>
#include <stdio.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct elpac_ctx elpac_ctx;

/* Create a context owning a copy of aid (freed by elpac_free).
 * Returns NULL on allocation failure. */
elpac_ctx *elpac_new(const uint8_t *aid, uint8_t aid_len);

/* Attach APDU/HTTP callbacks. apdu_token/http_token are opaque integers
 * handed back to the exported Go trampoline functions. */
int elpac_set_interfaces(elpac_ctx *c, uintptr_t apdu_token, uintptr_t http_token);

/* euicc_init; returns 0 on success. */
int elpac_init(elpac_ctx *c);

/* euicc_fini; call only after successful elpac_init. */
void elpac_fini(elpac_ctx *c);

/* euicc_http_cleanup; safe to call only when http state was set up. */
void elpac_http_cleanup(elpac_ctx *c);

/* Full teardown: clears interface pointers (callbacks unreachable),
 * frees interface structs, context and AID copy. Does NOT call
 * euicc_fini; the caller runs elpac_fini first when initialized. */
void elpac_free(elpac_ctx *c);

/* Toggle APDU/HTTP debug logging to stderr. */
void elpac_set_debug(elpac_ctx *c, int apdu_debug, int http_debug);

/* --- Read-only operations (P3) ---------------------------------- */

/* EID as lowercase hex string; caller frees with elpac_string_free. */
int elpac_get_eid(elpac_ctx *c, char **eid_out);

/* Configured addresses; either output may be NULL if unset.
 * Caller frees non-NULL strings with elpac_string_free. */
int elpac_get_configured_addresses(elpac_ctx *c, char **default_dp_out, char **root_ds_out);

/* EUICCInfo2 (ES10c GetEuiccInfo2 subset); zero-filled on failure. */
struct elpac_euiccinfo2 {
    char *profileVersion;
    char *svn;
    char *euiccFirmwareVer;
    uint32_t installedApplication;
    uint32_t freeNonVolatileMemory;
    uint32_t freeVolatileMemory;
    const char **uiccCapability; /* NULL-terminated */
    char *ts102241Version;
    char *globalplatformVersion;
    const char **rspCapability; /* NULL-terminated */
    const char **euiccCiPKIdListForVerification; /* NULL-terminated */
    const char **euiccCiPKIdListForSigning;      /* NULL-terminated */
    char *euiccCategory;
    const char **forbiddenProfilePolicyRules; /* NULL-terminated */
    char *ppVersion;
    char *sasAcreditationNumber;
    char *platformLabel;
    char *discoveryBaseURL;
};
int elpac_get_euiccinfo2(elpac_ctx *c, struct elpac_euiccinfo2 *out);
void elpac_euiccinfo2_free(struct elpac_euiccinfo2 *info);

/* Profile list (ES10c GetProfilesInfo), flattened array. */
struct elpac_profile {
    const char *iccid;
    const char *isdpAid;
    int32_t profileState; /* raw es10c_profile_state */
    int32_t profileClass; /* raw es10c_profile_class */
    const char *profileNickname;     /* NULL if absent */
    const char *serviceProviderName; /* NULL if absent */
    const char *profileName;         /* NULL if absent */
    int32_t iconType;                /* raw es10c_icon_type */
    const char *iconBase64;          /* NULL if absent */
};
struct elpac_profile_list {
    struct elpac_profile *items;
    uint32_t count;
};
int elpac_get_profiles(elpac_ctx *c, struct elpac_profile_list *out);
void elpac_profile_list_free(struct elpac_profile_list *list);

/* Notification list (ES10b ListNotification), flattened array. */
struct elpac_notification {
    uint64_t seqNumber;
    int32_t profileManagementOperation; /* raw es10b_profile_management_operation */
    const char *notificationAddress;    /* NULL if absent */
    const char *iccid;                  /* NULL if absent */
};
struct elpac_notification_list {
    struct elpac_notification *items;
    uint32_t count;
};
int elpac_get_notifications(elpac_ctx *c, struct elpac_notification_list *out);
void elpac_notification_list_free(struct elpac_notification_list *list);

void elpac_string_free(char *s);

/* --- Write operations (P5) --------------------------------------- */

/* refreshFlag semantics follow libeuicc: nonzero = 0xFF (refresh). */
int elpac_profile_enable(elpac_ctx *c, const char *iccid, int refresh);
int elpac_profile_disable(elpac_ctx *c, const char *iccid, int refresh);
int elpac_profile_delete(elpac_ctx *c, const char *iccid);
int elpac_set_nickname(elpac_ctx *c, const char *iccid, const char *nickname);
int elpac_set_default_dp(elpac_ctx *c, const char *smdp);

/* Retrieve one notification: address and base64 payload (caller frees
 * both with elpac_string_free). */
int elpac_notification_retrieve(elpac_ctx *c, uint64_t seq, char **address_out, char **b64_out);
/* Send a pending notification to the currently configured server. */
int elpac_notification_send(elpac_ctx *c, const char *b64_pending);
/* ES10b NotificationSent; nonzero return = failure. */
int elpac_notification_remove(elpac_ctx *c, uint64_t seq);

/* --- Download orchestration (P5) --------------------------------- */

/* Set the SM-DP+ address (C copies the string). */
int elpac_set_server_address(elpac_ctx *c, const char *smdp);

int elpac_download_get_challenge_and_info(elpac_ctx *c);
int elpac_download_initiate_authentication(elpac_ctx *c);
int elpac_download_authenticate_server(elpac_ctx *c, const char *matching_id, const char *imei);
int elpac_download_authenticate_client(elpac_ctx *c);
int elpac_download_prepare_download(elpac_ctx *c, const char *confirmation_code);
int elpac_download_get_bound_profile_package(elpac_ctx *c);
int elpac_download_cancel(elpac_ctx *c, int reason);

struct elpac_download_result {
    uint64_t seq_number;
    int32_t bpp_command_id;
    int32_t error_reason;
    char *iccid; /* may be NULL */
};
int elpac_download_load_bound_profile_package(elpac_ctx *c, struct elpac_download_result *out);
void elpac_download_result_free(struct elpac_download_result *result);

/* Profile metadata from the downloaded package (preview step). */
struct elpac_profile_metadata {
    char *iccid;
    char *serviceProviderName;
    char *profileName;
    int32_t iconType;
    char *iconBase64;
    int32_t profileClass;
};
int elpac_download_metadata(elpac_ctx *c, struct elpac_profile_metadata *out);
void elpac_profile_metadata_free(struct elpac_profile_metadata *meta);

/* Last ES9+ status (subject/reason codes) as NUL-terminated strings;
 * valid until the next operation on the context. Never NULL. */
const char *elpac_http_status_subject_code(elpac_ctx *c);
const char *elpac_http_status_reason_code(elpac_ctx *c);
const char *elpac_http_status_message(elpac_ctx *c);
/* Nonzero when the last ES9+ status reported an error (codes set and
 * not the 0.0.0 defaults). */
int elpac_http_status_is_error(elpac_ctx *c);


#ifdef __cplusplus
}
#endif

#endif /* ELPAC_SHIM_H */
