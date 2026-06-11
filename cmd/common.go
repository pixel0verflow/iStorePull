package cmd

import (
	"context"
	"fmt"

	"github.com/pixel0verflow/istorepull/pkg/itunes"
)

// resolveAdamID returns the adam id from an explicit id, or by looking up a
// bundle id against the public iTunes API.
func resolveAdamID(ctx context.Context, bundleID string, adamID int64, country string) (int64, error) {
	switch {
	case adamID != 0 && bundleID != "":
		return 0, fmt.Errorf("%w: specify only one of -b/--bundle or -i/--id", errBadInput)
	case adamID != 0:
		return adamID, nil
	case bundleID != "":
		app, err := itunes.New().LookupBundle(ctx, bundleID, country)
		if err != nil {
			return 0, fmt.Errorf("resolve bundle %s: %w", bundleID, err)
		}
		if app.ID == 0 {
			return 0, fmt.Errorf("%w: no adam id for bundle %s", errBadInput, bundleID)
		}
		return app.ID, nil
	default:
		return 0, fmt.Errorf("%w: specify -b/--bundle or -i/--id", errBadInput)
	}
}
