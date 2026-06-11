package cmd

import (
	"fmt"

	"github.com/pixel0verflow/istorepull/pkg/store"
	"github.com/spf13/cobra"
)

func newVersionsCmd() *cobra.Command {
	var (
		bundleID string
		adamID   int64
		country  string
		resolve  bool
		last     int
	)
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "List downloadable build version ids for a title",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			id, err := resolveAdamID(ctx, bundleID, adamID, country)
			if err != nil {
				return err
			}
			sess, err := loadSession()
			if err != nil {
				return err
			}
			client, err := store.New(sess)
			if err != nil {
				return err
			}
			vl, err := client.Versions(ctx, id)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "adamId:  %d\nlatest:  %s\nbuilds:  %d\n", vl.AdamID, vl.Latest, len(vl.ExternalIDs))

			if !resolve {
				for _, e := range vl.ExternalIDs {
					fmt.Fprintln(out, e)
				}
				return nil
			}
			return printResolved(cmd, client, vl, last)
		},
	}
	cmd.Flags().StringVarP(&bundleID, "bundle", "b", "", "bundle id")
	cmd.Flags().Int64VarP(&adamID, "id", "i", 0, "adam id")
	cmd.Flags().StringVar(&country, "country", "", "storefront country for bundle lookup")
	cmd.Flags().BoolVar(&resolve, "resolve", false, "probe each id to print its version string")
	cmd.Flags().IntVar(&last, "last", 0, "with --resolve, only resolve the newest N ids")
	return cmd
}

func printResolved(cmd *cobra.Command, client store.Client, vl store.VersionList, last int) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	ids := vl.ExternalIDs
	if last > 0 && last < len(ids) {
		ids = ids[len(ids)-last:]
	}

	cache := loadVermap(vl.AdamID)
	var toProbe []string
	for _, id := range ids {
		if _, ok := cache[id]; !ok {
			toProbe = append(toProbe, id)
		}
	}
	if len(toProbe) > 0 {
		infos, err := client.ResolveVersions(ctx, vl, toProbe)
		if err != nil {
			return err
		}
		for _, in := range infos {
			cache[in.ExternalID] = in.Version
		}
		saveVermap(vl.AdamID, cache)
	}

	fmt.Fprintf(out, "\n%-14s %s\n", "EXTERNAL-ID", "VERSION")
	for _, id := range ids {
		fmt.Fprintf(out, "%-14s %s\n", id, cache[id])
	}
	return nil
}
