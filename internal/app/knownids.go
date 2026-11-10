package app

import (
	"context"

	"EasyLPAC/internal/lpa"
)

// knownIDs records in the log every EID, ICCID and IMEI that goes
// through the service, as soon as it does, so that obscured entries hide
// them in raw protocol data, the entries logged before included.
type knownIDs struct {
	lpa.Service
	log *LogBuffer
}

func (k knownIDs) ChipInfo(ctx context.Context) (*lpa.EuiccInfo, error) {
	info, err := k.Service.ChipInfo(ctx)
	if info != nil {
		k.log.AddIdentifiers(info.EidValue)
	}
	return info, err
}

func (k knownIDs) ProfileList(ctx context.Context) ([]*lpa.Profile, error) {
	list, err := k.Service.ProfileList(ctx)
	for _, p := range list {
		k.log.AddIdentifiers(p.Iccid)
	}
	return list, err
}

func (k knownIDs) NotificationList(ctx context.Context) ([]*lpa.Notification, error) {
	list, err := k.Service.NotificationList(ctx)
	for _, n := range list {
		k.log.AddIdentifiers(n.Iccid)
	}
	return list, err
}

// ProfileDownload records the IMEI given, and the ICCID of the profile
// once its preview tells it, before the package that holds it is
// fetched.
func (k knownIDs) ProfileDownload(ctx context.Context, req lpa.DownloadRequest) (*lpa.DownloadOutcome, error) {
	k.log.AddIdentifiers(req.IMEI)
	if confirm := req.Confirm; confirm != nil {
		req.Confirm = func(meta lpa.ProfileMetadata) (bool, error) {
			k.log.AddIdentifiers(meta.ICCID)
			return confirm(meta)
		}
	}
	out, err := k.Service.ProfileDownload(ctx, req)
	if out != nil && out.Notification != nil {
		k.log.AddIdentifiers(out.Notification.Iccid)
	}
	return out, err
}
