package store

import "strconv"

// downloadPayload is the plist body of a volumeStoreDownloadProduct request.
type downloadPayload struct {
	CreditDisplay     string `plist:"creditDisplay"`
	GUID              string `plist:"guid"`
	SalableAdamID     int64  `plist:"salableAdamId"`
	PricingParameters string `plist:"pricingParameters"`
	ExternalVersionID int64  `plist:"externalVersionId,omitempty"`
}

// newDownloadPayload builds the request body. An empty externalVersionID asks
// for the current build (the field is omitted).
func newDownloadPayload(guid string, adamID int64, externalVersionID string) downloadPayload {
	p := downloadPayload{
		CreditDisplay:     "",
		GUID:              guid,
		SalableAdamID:     adamID,
		PricingParameters: "STDQ",
	}
	if externalVersionID != "" {
		if id, err := strconv.ParseInt(externalVersionID, 10, 64); err == nil {
			p.ExternalVersionID = id
		}
	}
	return p
}
