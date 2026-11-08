package euicc

/*
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"encoding/base64"
	"errors"
	"unsafe"
)

// ProfileRefreshFlag mirrors libeuicc's refresh semantics.
const (
	RefreshNo  = 0
	RefreshYes = 1
)

// EnableProfile enables a profile by ICCID. refresh requests the
// eUICC/modem refresh procedure (libeuicc maps it to 0xFF).
func (c *Context) EnableProfile(iccid string, refresh int) error {
	cICCID := C.CString(iccid)
	defer C.free(unsafe.Pointer(cICCID))
	if rc := C.elpac_profile_enable(c.h, cICCID, C.int(refresh)); rc != 0 {
		return c.failed("es10c_enable_profile")
	}
	return nil
}

// DisableProfile disables a profile by ICCID.
func (c *Context) DisableProfile(iccid string, refresh int) error {
	cICCID := C.CString(iccid)
	defer C.free(unsafe.Pointer(cICCID))
	if rc := C.elpac_profile_disable(c.h, cICCID, C.int(refresh)); rc != 0 {
		return c.failed("es10c_disable_profile")
	}
	return nil
}

// DeleteProfile deletes a disabled profile by ICCID.
func (c *Context) DeleteProfile(iccid string) error {
	cICCID := C.CString(iccid)
	defer C.free(unsafe.Pointer(cICCID))
	if rc := C.elpac_profile_delete(c.h, cICCID); rc != 0 {
		return c.failed("es10c_delete_profile")
	}
	return nil
}

// SetNickname sets or clears (empty string) the profile nickname.
func (c *Context) SetNickname(iccid, nickname string) error {
	cICCID := C.CString(iccid)
	defer C.free(unsafe.Pointer(cICCID))
	cNick := C.CString(nickname)
	defer C.free(unsafe.Pointer(cNick))
	if rc := C.elpac_set_nickname(c.h, cICCID, cNick); rc != 0 {
		return c.failed("es10c_set_nickname")
	}
	return nil
}

// SetDefaultDpAddress stores the default SM-DP+ address on the card.
func (c *Context) SetDefaultDpAddress(smdp string) error {
	cSMDP := C.CString(smdp)
	defer C.free(unsafe.Pointer(cSMDP))
	if rc := C.elpac_set_default_dp(c.h, cSMDP); rc != 0 {
		return c.failed("es10a_set_default_dp_address")
	}
	return nil
}

// PendingNotification is one retrieved notification.
type PendingNotification struct {
	Address       string
	Base64Payload string
}

// RetrieveNotification fetches one pending notification from the card.
func (c *Context) RetrieveNotification(seq uint64) (*PendingNotification, error) {
	var address, b64 *C.char
	if rc := C.elpac_notification_retrieve(c.h, C.uint64_t(seq), &address, &b64); rc != 0 {
		return nil, c.failed("es10b_retrieve_notifications_list")
	}
	defer C.elpac_string_free(address)
	defer C.elpac_string_free(b64)
	return &PendingNotification{
		Address:       goStringOwned(address),
		Base64Payload: goStringOwned(b64),
	}, nil
}

// SendNotification delivers a pending notification to the SM-DP+ that
// the notification is addressed to (set via SetServerAddress).
func (c *Context) SendNotification(base64Payload string) error {
	cPayload := C.CString(base64Payload)
	defer C.free(unsafe.Pointer(cPayload))
	if rc := C.elpac_notification_send(c.h, cPayload); rc != 0 {
		return c.failed("es9p_handle_notification")
	}
	return nil
}

// RemoveNotification deletes a notification from the card list after
// it was sent successfully.
func (c *Context) RemoveNotification(seq uint64) error {
	if rc := C.elpac_notification_remove(c.h, C.uint64_t(seq)); rc != 0 {
		return c.failed("es10b_remove_notification_from_list")
	}
	return nil
}

// HTTPStatus is the last ES9+ status reported by libeuicc.
type HTTPStatus struct {
	SubjectCode string
	ReasonCode  string
	Message     string
}

// IsError reports whether the last ES9+ status carried error codes.
func (s HTTPStatus) IsError() bool {
	return !(s.SubjectCode == "0.0.0" && s.ReasonCode == "0.0.0") &&
		(s.SubjectCode != "" || s.ReasonCode != "")
}

// HTTPStatus returns the status of the most recent ES9+ transaction.
func (c *Context) HTTPStatus() HTTPStatus {
	return HTTPStatus{
		SubjectCode: C.GoString(C.elpac_http_status_subject_code(c.h)),
		ReasonCode:  C.GoString(C.elpac_http_status_reason_code(c.h)),
		Message:     C.GoString(C.elpac_http_status_message(c.h)),
	}
}

// SetServerAddress sets the SM-DP+/SM-DS address used by ES9+/ES11.
func (c *Context) SetServerAddress(smdp string) error {
	cSMDP := C.CString(smdp)
	defer C.free(unsafe.Pointer(cSMDP))
	if rc := C.elpac_set_server_address(c.h, cSMDP); rc != 0 {
		return errors.New("euicc: set server address failed")
	}
	return nil
}

// --- download stages -------------------------------------------------

func (c *Context) DownloadGetChallengeAndInfo() error {
	if rc := C.elpac_download_get_challenge_and_info(c.h); rc != 0 {
		return c.failed("es10b_get_euicc_challenge_and_info")
	}
	return nil
}

func (c *Context) DownloadInitiateAuthentication() error {
	if rc := C.elpac_download_initiate_authentication(c.h); rc != 0 {
		return c.failed("es9p_initiate_authentication")
	}
	return nil
}

func (c *Context) DownloadAuthenticateServer(matchingID, imei string) error {
	var cMatching, cIMEI *C.char
	if matchingID != "" {
		cMatching = C.CString(matchingID)
		defer C.free(unsafe.Pointer(cMatching))
	}
	if imei != "" {
		cIMEI = C.CString(imei)
		defer C.free(unsafe.Pointer(cIMEI))
	}
	if rc := C.elpac_download_authenticate_server(c.h, cMatching, cIMEI); rc != 0 {
		return c.failed("es10b_authenticate_server")
	}
	return nil
}

func (c *Context) DownloadAuthenticateClient() error {
	if rc := C.elpac_download_authenticate_client(c.h); rc != 0 {
		return c.failed("es9p_authenticate_client")
	}
	return nil
}

// ProfileMetadata is the preview of the profile to be installed.
type ProfileMetadata struct {
	ICCID               string
	ServiceProviderName string
	ProfileName         string
	IconType            int32
	Icon                []byte
	Class               int32
}

// DownloadMetadata parses the profile metadata received during
// authenticateClient (preview step).
func (c *Context) DownloadMetadata() (*ProfileMetadata, error) {
	var meta C.struct_elpac_profile_metadata
	if rc := C.elpac_download_metadata(c.h, &meta); rc != 0 {
		return nil, errors.New("euicc: profile metadata unavailable")
	}
	defer C.elpac_profile_metadata_free(&meta)

	out := &ProfileMetadata{
		ICCID:               goStringOwned(meta.iccid),
		ServiceProviderName: goStringOwned(meta.serviceProviderName),
		ProfileName:         goStringOwned(meta.profileName),
		IconType:            int32(meta.iconType),
		Class:               int32(meta.profileClass),
	}
	if meta.iconBase64 != nil {
		if raw, err := base64.StdEncoding.DecodeString(C.GoString(meta.iconBase64)); err == nil {
			out.Icon = raw
		}
	}
	return out, nil
}

func (c *Context) DownloadPrepare(confirmationCode string) error {
	var cCode *C.char
	if confirmationCode != "" {
		cCode = C.CString(confirmationCode)
		defer C.free(unsafe.Pointer(cCode))
	}
	if rc := C.elpac_download_prepare_download(c.h, cCode); rc != 0 {
		return c.failed("es10b_prepare_download")
	}
	return nil
}

func (c *Context) DownloadGetBoundProfilePackage() error {
	if rc := C.elpac_download_get_bound_profile_package(c.h); rc != 0 {
		return c.failed("es9p_get_bound_profile_package")
	}
	return nil
}

// DownloadResult reports the outcome of the BPP install.
type DownloadResult struct {
	SeqNumber    uint64
	BppCommandID int32
	ErrorReason  int32
	ICCID        string
}

// CancelSessionReason mirrors ES10b cancel reasons.
type CancelSessionReason int

const (
	CancelEndUserRejection CancelSessionReason = 0
	CancelPostponed        CancelSessionReason = 1
	CancelTimeout          CancelSessionReason = 2
	CancelPPRNotAllowed    CancelSessionReason = 3
	CancelMetadataMismatch CancelSessionReason = 4
	CancelLoadBPPError     CancelSessionReason = 5
)

// DownloadCancel runs ES10b CancelSession plus ES9+ cancelSession on a
// best-effort basis; the caller keeps the original error regardless.
func (c *Context) DownloadCancel(reason CancelSessionReason) error {
	if rc := C.elpac_download_cancel(c.h, C.int(reason)); rc != 0 {
		return errors.New("euicc: cancel session failed")
	}
	return nil
}

// DownloadLoadBoundProfilePackage installs the fetched package. On
// failure the returned DownloadResult carries the BPP command id and
// error reason reported by the card (state is known: the install did
// not complete).
func (c *Context) DownloadLoadBoundProfilePackage() (*DownloadResult, error) {
	var res C.struct_elpac_download_result
	rc := C.elpac_download_load_bound_profile_package(c.h, &res)
	out := &DownloadResult{
		SeqNumber:    uint64(res.seq_number),
		BppCommandID: int32(res.bpp_command_id),
		ErrorReason:  int32(res.error_reason),
		ICCID:        goStringOwned(res.iccid),
	}
	C.elpac_download_result_free(&res)
	if rc != 0 {
		return out, c.failed("es10b_load_bound_profile_package")
	}
	return out, nil
}
