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
		last     int
		all      bool
	)
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "List downloadable build versions for a title",
		Long: "versions lists the builds Apple still serves for a title. By default it\n" +
			"resolves the newest few into human version strings (one store request per\n" +
			"version, cached afterwards). Use --all to dump every raw external id cheaply.",
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
			fmt.Fprintf(out, "adamId:  %d\nbuilds:  %d total\n", vl.AdamID, len(vl.ExternalIDs))

			if all {
				fmt.Fprintln(out, "\nall external version ids (oldest → newest):")
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
	cmd.Flags().IntVar(&last, "last", 5, "how many of the newest builds to resolve into version strings")
	cmd.Flags().BoolVar(&all, "all", false, "list every raw external version id instead (no resolving)")
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
		fmt.Fprintf(out, "resolving newest %d (one store request each)…\n", len(toProbe))
		infos, err := client.ResolveVersions(ctx, vl, toProbe)
		if err != nil {
			return err
		}
		for _, in := range infos {
			cache[in.ExternalID] = in.Version
		}
		saveVermap(vl.AdamID, cache)
	}

	fmt.Fprintf(out, "\n%-10s %-14s %s\n", "VERSION", "EXTERNAL-ID", "")
	// newest first for readability
	for i := len(ids) - 1; i >= 0; i-- {
		id := ids[i]
		ver := cache[id]
		if ver == "" {
			ver = "?"
		}
		marker := ""
		if id == vl.Latest {
			marker = "(current)"
		}
		fmt.Fprintf(out, "%-10s %-14s %s\n", ver, id, marker)
	}
	if len(vl.ExternalIDs) > len(ids) {
		fmt.Fprintf(out, "\n…and %d older builds. Use --last N for more, or --all for every id.\n", len(vl.ExternalIDs)-len(ids))
	}
	return nil
}
