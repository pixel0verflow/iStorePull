package cmd

import (
	"fmt"

	"github.com/pixel0verflow/istorepull/pkg/itunes"
	"github.com/spf13/cobra"
)

func newLookupCmd() *cobra.Command {
	var (
		bundleID string
		adamID   int64
		country  string
	)
	cmd := &cobra.Command{
		Use:   "lookup",
		Short: "Resolve a bundle id <-> adam id via the public iTunes API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (bundleID == "") == (adamID == 0) {
				return fmt.Errorf("%w: specify exactly one of -b/--bundle or -i/--id", errBadInput)
			}
			c := itunes.New()
			var (
				app itunes.App
				err error
			)
			if bundleID != "" {
				app, err = c.LookupBundle(cmd.Context(), bundleID, country)
			} else {
				app, err = c.LookupID(cmd.Context(), adamID, country)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "adamId:   %d\nbundleId: %s\nname:     %s\nversion:  %s\nseller:   %s\n",
				app.ID, app.BundleID, app.Name, app.Version, app.Seller)
			return nil
		},
	}
	cmd.Flags().StringVarP(&bundleID, "bundle", "b", "", "bundle id")
	cmd.Flags().Int64VarP(&adamID, "id", "i", 0, "adam id")
	cmd.Flags().StringVar(&country, "country", "", "storefront country code (e.g. us, pl)")
	return cmd
}

func newSearchCmd() *cobra.Command {
	var (
		limit   int
		country string
	)
	cmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search the public iTunes App Store",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			term := args[0]
			for _, a := range args[1:] {
				term += " " + a
			}
			apps, err := itunes.New().Search(cmd.Context(), term, country, limit)
			if err != nil {
				return err
			}
			for _, a := range apps {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12d %-40s %s\n", a.ID, truncate(a.BundleID, 40), a.Name)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "max results")
	cmd.Flags().StringVar(&country, "country", "", "storefront country code")
	return cmd
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
