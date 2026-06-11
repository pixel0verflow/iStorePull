package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
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

// FindExternalID resolves a human version string to its external version id.
//
// The external id list is chronological and build versions are ~monotonically
// increasing, so it binary-searches (≈log2(n) probes) instead of scanning all
// builds. If the search lands on a build Apple no longer serves it probes the
// nearest served neighbour. If the list turns out non-monotonic and the search
// misses, it falls back to a linear newest-first scan for correctness.
func (c *client) FindExternalID(ctx context.Context, vl VersionList, version string) (string, error) {
	target := parseVersion(version)
	lo, hi := 0, len(vl.ExternalIDs)-1
	for lo <= hi {
		idx, ver, err := c.probeNearestServed(ctx, vl, lo, hi)
		if err != nil {
			break // no served build in range, or hard error → fall back
		}
		if ver == version {
			return vl.ExternalIDs[idx], nil
		}
		if compareVersions(parseVersion(ver), target) < 0 {
			lo = idx + 1
		} else {
			hi = idx - 1
		}
	}
	return c.findLinear(ctx, vl, version)
}

// probeNearestServed probes the midpoint of [lo,hi]; if that build is no longer
// served it expands outward to the nearest served build within the range.
// Returns the served build's index and version string.
func (c *client) probeNearestServed(ctx context.Context, vl VersionList, lo, hi int) (int, string, error) {
	mid := (lo + hi) / 2
	for offset := 0; ; offset++ {
		hit := false
		for _, idx := range []int{mid + offset, mid - offset} {
			if idx < lo || idx > hi {
				continue
			}
			hit = true
			item, err := c.DownloadProduct(ctx, vl.AdamID, vl.ExternalIDs[idx])
			if err != nil {
				if isSkippable(err) {
					continue
				}
				return 0, "", err
			}
			return idx, item.Version, nil
		}
		if !hit {
			return 0, "", ErrNotServed
		}
	}
}

// findLinear scans newest-first as a correctness fallback.
func (c *client) findLinear(ctx context.Context, vl VersionList, version string) (string, error) {
	for i := len(vl.ExternalIDs) - 1; i >= 0; i-- {
		item, err := c.DownloadProduct(ctx, vl.AdamID, vl.ExternalIDs[i])
		if err != nil {
			if isSkippable(err) {
				continue
			}
			return "", err
		}
		if item.Version == version {
			return vl.ExternalIDs[i], nil
		}
	}
	return "", ErrNotServed
}

// parseVersion splits a dotted version string into integer components
// (non-numeric parts become 0).
func parseVersion(v string) []int {
	parts := strings.Split(v, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(strings.TrimSpace(p))
		out[i] = n
	}
	return out
}

// compareVersions compares two parsed versions component-wise; missing trailing
// components count as 0. Returns -1, 0 or 1.
func compareVersions(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}
	return 0
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
