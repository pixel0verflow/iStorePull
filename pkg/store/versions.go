package store

import (
	"context"
	"errors"
	"time"
)

// probeDelay throttles version-resolution probes to avoid burning the token.
var probeDelay = 150 * time.Millisecond

// Versions lists downloadable builds by reading the version id list off a
// current product response's metadata.
func (c *client) Versions(ctx context.Context, adamID int64) (VersionList, error) {
	item, err := c.DownloadProduct(ctx, adamID, "")
	if err != nil {
		return VersionList{}, err
	}
	vl := VersionList{
		AdamID:      adamID,
		Latest:      item.ExtVerID,
		ExternalIDs: externalIDs(item.Metadata),
	}
	return vl, nil
}

// externalIDs reads softwareVersionExternalIdentifiers as a []string.
func externalIDs(meta map[string]any) []string {
	raw, ok := meta["softwareVersionExternalIdentifiers"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		switch n := v.(type) {
		case int64:
			out = append(out, itoa(n))
		case uint64:
			out = append(out, itoa(int64(n)))
		case int:
			out = append(out, itoa(int64(n)))
		case string:
			out = append(out, n)
		}
	}
	return out
}

// FindExternalID resolves a human version string to its external version id by
// probing candidate ids newest-first. Returns ErrNotServed if no served build
// matches.
func (c *client) FindExternalID(ctx context.Context, vl VersionList, version string) (string, error) {
	for i := len(vl.ExternalIDs) - 1; i >= 0; i-- {
		id := vl.ExternalIDs[i]
		item, err := c.DownloadProduct(ctx, vl.AdamID, id)
		if err != nil {
			if isSkippable(err) {
				continue
			}
			return "", err
		}
		if item.Version == version {
			return id, nil
		}
	}
	return "", ErrNotServed
}

// isSkippable reports whether a per-build error should be skipped during a scan
// (build no longer served) rather than aborting the whole resolution.
func isSkippable(err error) bool {
	return errors.Is(err, ErrNotServed) || errors.Is(err, ErrUnavailable) || errors.Is(err, ErrEmptyResult)
}

// ResolveVersions probes external ids and returns their version strings. When
// only is nil the whole list is resolved; otherwise just the listed ids.
func (c *client) ResolveVersions(ctx context.Context, vl VersionList, only []string) ([]VersionInfo, error) {
	ids := only
	if ids == nil {
		ids = vl.ExternalIDs
	}
	out := make([]VersionInfo, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			case <-time.After(probeDelay):
			}
		}
		item, err := c.DownloadProduct(ctx, vl.AdamID, id)
		if err != nil {
			// Skip builds Apple no longer serves; keep resolving the rest.
			if isSkippable(err) {
				continue
			}
			return out, err
		}
		out = append(out, VersionInfo{ExternalID: id, Version: item.Version})
	}
	return out, nil
}
