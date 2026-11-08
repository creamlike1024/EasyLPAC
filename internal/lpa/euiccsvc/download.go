package euiccsvc

// Write paths and the download/notification orchestration of the
// libeuicc service (P5).

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"EasyLPAC/internal/euicc"
	"EasyLPAC/internal/lpa"
)

// IsUserCancelled reports whether err came from a declined preview.
func IsUserCancelled(err error) bool { return lpa.IsUserCancelled(err) }

// --- write operations ------------------------------------------------

func (s *Service) ProfileEnable(ctx context.Context, iccid string) error {
	return s.write(ctx, lpa.StageProfileEnable, "profile-enable", func(c *euicc.Context) error {
		return c.EnableProfile(iccid, euicc.RefreshYes)
	})
}

func (s *Service) ProfileDisable(ctx context.Context, iccid string) error {
	return s.write(ctx, lpa.StageProfileDisable, "profile-disable", func(c *euicc.Context) error {
		return c.DisableProfile(iccid, euicc.RefreshYes)
	})
}

func (s *Service) ProfileDelete(ctx context.Context, iccid string) error {
	return s.write(ctx, lpa.StageProfileDelete, "profile-delete", func(c *euicc.Context) error {
		return c.DeleteProfile(iccid)
	})
}

func (s *Service) ProfileNickname(ctx context.Context, iccid, nickname string) error {
	return s.write(ctx, lpa.StageProfileNickname, "profile-nickname", func(c *euicc.Context) error {
		return c.SetNickname(iccid, nickname)
	})
}

func (s *Service) ChipDefaultSmdp(ctx context.Context, smdp string) error {
	return s.write(ctx, lpa.StageSetDefaultSmdp, "set-default-smdp", func(c *euicc.Context) error {
		return c.SetDefaultDpAddress(smdp)
	})
}

// write runs one state-changing operation. The card state is only
// known for reported failures (libeuicc returns the card's own error);
// callers must not auto-retry destructive operations on unknown state.
func (s *Service) write(ctx context.Context, stage lpa.Stage, op string, fn func(*euicc.Context) error) error {
	err := s.withContext(ctx, fn)
	if err != nil {
		return s.wrapOp(stage, op, err)
	}
	return nil
}

// --- notification processing ----------------------------------------

// NotificationProcess implements the read/send/remove sequence:
// retrieve the pending notification, deliver it to its own address
// (regardless of the currently enabled profile) and optionally remove
// it from the card after a successful send.
func (s *Service) NotificationProcess(ctx context.Context, seq int, remove bool) error {
	err := s.withContext(ctx, func(c *euicc.Context) error {
		pending, err := c.RetrieveNotification(uint64(seq))
		if err != nil {
			return err
		}
		if err := c.SetServerAddress(pending.Address); err != nil {
			return err
		}
		if err := c.SendNotification(pending.Base64Payload); err != nil {
			return fmt.Errorf("%w: %s", err, c.HTTPStatus().Message)
		}
		if remove {
			if err := c.RemoveNotification(uint64(seq)); err != nil {
				// The notification was delivered; removal failing is
				// a separate, non-destructive failure.
				return fmt.Errorf("%w: %v", ErrNotificationSentRemovalFailed, err)
			}
		}
		return nil
	})
	if err != nil {
		return s.wrapOp(lpa.StageNotifyProcess, "notification-process", err)
	}
	return nil
}

// NotificationRemove deletes a notification without sending it.
func (s *Service) NotificationRemove(ctx context.Context, seq int) error {
	err := s.withContext(ctx, func(c *euicc.Context) error {
		return c.RemoveNotification(uint64(seq))
	})
	if err != nil {
		return s.wrapOp(lpa.StageNotifyRemove, "notification-remove", err)
	}
	return nil
}

// --- download --------------------------------------------------------

// downloadStages are reported through Config.OnStage in order.
const (
	stageChallenge        = "es10b_get_euicc_challenge_and_info"
	stageInitiateAuth     = "es9p_initiate_authentication"
	stageAuthenticateSrv  = "es10b_authenticate_server"
	stageAuthenticateCli  = "es9p_authenticate_client"
	stagePreview          = "preview"
	stagePrepareDownload  = "es10b_prepare_download"
	stageGetBoundPackage  = "es9p_get_bound_profile_package"
	stageLoadBoundPackage = "es10b_load_bound_profile_package"
	stageCancel           = "cancel"
)

// ProfileDownload runs the ES9+/ES10b install flow:
//
//	challenge+info -> initiateAuthentication -> authenticateServer ->
//	authenticateClient -> [preview] -> prepareDownload ->
//	getBoundProfilePackage -> loadBoundProfilePackage
//
// Any failure before the install completes attempts a protocol cancel
// (ES10b CancelSession + ES9+ cancelSession) as best effort; the
// original error is preserved. A failure during loadBoundProfilePackage
// is never retried automatically, and the returned error carries the
// reported BPP command id / error reason so the card state is known.
func (s *Service) ProfileDownload(ctx context.Context, req lpa.DownloadRequest) (*lpa.DownloadOutcome, error) {
	var outcome *lpa.DownloadOutcome
	err := s.withContext(ctx, func(c *euicc.Context) error {
		smdp := req.SMDP
		if smdp == "" {
			// Fall back to the card's configured default SM-DP+.
			addr, err := c.GetConfiguredAddresses()
			if err != nil {
				return fmt.Errorf("no SM-DP+ given and configured addresses unavailable: %w", err)
			}
			smdp = addr.DefaultDpAddress
		}
		if smdp == "" {
			return errors.New("no SM-DP+ address available")
		}
		if err := c.SetServerAddress(smdp); err != nil {
			return err
		}

		stage := func(name string) { s.stage(name) }
		// Cancellation is checked between stages: an in-flight APDU or
		// HTTP call cannot be interrupted, but the flow stops before
		// starting the next one.
		step := func(fn func() error, name string) error {
			if err := ctx.Err(); err != nil {
				s.downloadCancel(c)
				return err
			}
			if err := fn(); err != nil {
				return s.downloadFail(c, name, err)
			}
			return s.checkHTTPStatus(c, name)
		}

		stage(stageChallenge)
		if err := step(c.DownloadGetChallengeAndInfo, ""); err != nil {
			return err
		}
		stage(stageInitiateAuth)
		if err := step(c.DownloadInitiateAuthentication, stageInitiateAuth); err != nil {
			return err
		}
		stage(stageAuthenticateSrv)
		if err := step(func() error { return c.DownloadAuthenticateServer(req.MatchID, req.IMEI) }, ""); err != nil {
			return err
		}
		stage(stageAuthenticateCli)
		if err := step(c.DownloadAuthenticateClient, stageAuthenticateCli); err != nil {
			return err
		}

		if req.Confirm != nil {
			stage(stagePreview)
			meta, err := c.DownloadMetadata()
			if err != nil {
				return s.downloadFail(c, stagePreview, err)
			}
			ok, err := req.Confirm(toDomainMetadata(meta))
			if err != nil {
				s.downloadCancel(c)
				return fmt.Errorf("preview rejected: %w", err)
			}
			if !ok {
				s.downloadCancel(c)
				return lpa.ErrUserCancelled
			}
		}

		stage(stagePrepareDownload)
		if err := step(func() error { return c.DownloadPrepare(req.ConfirmCode) }, ""); err != nil {
			return err
		}
		stage(stageGetBoundPackage)
		if err := step(c.DownloadGetBoundProfilePackage, stageGetBoundPackage); err != nil {
			return err
		}
		stage(stageLoadBoundPackage)
		result, err := c.DownloadLoadBoundProfilePackage()
		if err != nil {
			// The install did not complete; report the card's reason.
			return fmt.Errorf("%w (bppCommandId=%d errorReason=%d iccid=%s)",
				err, result.BppCommandID, result.ErrorReason, result.ICCID)
		}
		// The card reports the install notification it created; this is
		// the exact notification the caller must process afterwards.
		outcome = &lpa.DownloadOutcome{
			Notification: &lpa.Notification{
				SeqNumber:                  int(result.SeqNumber),
				ProfileManagementOperation: "install",
				Iccid:                      result.ICCID,
			},
		}
		return nil
	})
	if err != nil {
		return nil, s.wrapOp(lpa.StageDownload, "download", err)
	}
	return outcome, nil
}

// ErrNotificationSentRemovalFailed reports that the notification was
// delivered but removing it from the card failed. The UI must present
// this as a successful send with a cleanup problem, not as a failure.
var ErrNotificationSentRemovalFailed = errors.New("notification sent but removal failed")

// checkHTTPStatus fails an ES9+ stage whose status codes report an
// error even though libeuicc was able to parse the payload.
func (s *Service) checkHTTPStatus(c *euicc.Context, stage string) error {
	if status := c.HTTPStatus(); status.IsError() {
		return &lpa.ES9Error{Stage: stage, Subject: status.SubjectCode, Reason: status.ReasonCode, Message: status.Message}
	}
	return nil
}

// downloadFail attempts a protocol cancel and enriches the error with
// the ES9+ status when the failure happened in an HTTP stage.
func (s *Service) downloadFail(c *euicc.Context, stage string, err error) error {
	s.stage(stageCancel)
	s.downloadCancel(c)
	status := c.HTTPStatus()
	if status.SubjectCode != "" && status.SubjectCode != "0.0.0" {
		return &lpa.ES9Error{Stage: stage, Subject: status.SubjectCode, Reason: status.ReasonCode, Message: status.Message, Cause: err}
	}
	if strings.Contains(err.Error(), stage) {
		return err // libeuicc's error names the stage already
	}
	return fmt.Errorf("%s: %w", stage, err)
}

// downloadCancel runs ES10b+ES9+ cancel as best effort; failures here
// are reported through the stage hook only (the original error stands).
func (s *Service) downloadCancel(c *euicc.Context) {
	_ = c.DownloadCancel(euicc.CancelEndUserRejection)
}

func (s *Service) stage(name string) {
	if s.cfg.OnStage != nil {
		s.cfg.OnStage(name)
	}
}

func toDomainMetadata(meta *euicc.ProfileMetadata) lpa.ProfileMetadata {
	return lpa.ProfileMetadata{
		ICCID:               meta.ICCID,
		ServiceProviderName: meta.ServiceProviderName,
		ProfileName:         meta.ProfileName,
		IconType:            iconTypeToString(meta.IconType),
		Icon:                meta.Icon,
		ProfileClass:        classToString(meta.Class),
	}
}
