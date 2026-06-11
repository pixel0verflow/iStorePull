package cmd

import (
	"fmt"
	"time"

	"github.com/pixel0verflow/istorepull/pkg/capture"
	"github.com/pixel0verflow/istorepull/pkg/credential"
	"github.com/spf13/cobra"
)

func newCaptureCmd() *cobra.Command {
	var (
		timeout time.Duration
		addr    string
		keepCA  bool
		appleID string
	)
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Capture a session automatically via an embedded proxy (macOS)",
		Long: "capture runs a short-lived HTTPS proxy that intercepts only Apple's store\n" +
			"hosts (gsa.apple.com is passed through untouched), trusts a throwaway CA, and\n" +
			"points the system proxy at itself. Trigger any app download in Apple\n" +
			"Configurator and the session is extracted and saved automatically.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := capture.Run(cmd.Context(), capture.NewSystem(), capture.Options{
				Addr:    addr,
				Timeout: timeout,
				KeepCA:  keepCA,
				Verbose: flagVerbose,
			}, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if appleID != "" {
				sess.AppleID = appleID
			}
			path, err := sessionPath()
			if err != nil {
				return err
			}
			if err := credential.Save(path, sess); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\ncaptured and saved session to %s\n", path)
			printSession(cmd.OutOrStdout(), sess)
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "how long to wait for a store request")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:0", "proxy listen address")
	cmd.Flags().BoolVar(&keepCA, "keep-ca", false, "leave the throwaway CA trusted after capture")
	cmd.Flags().StringVar(&appleID, "apple-id", "", "account email to embed in iTunesMetadata (optional)")
	return cmd
}
