package cmd

import (
	"fmt"
	"io"

	"github.com/pixel0verflow/istorepull/pkg/charles"
	"github.com/pixel0verflow/istorepull/pkg/credential"
	"github.com/spf13/cobra"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage the captured Configurator session",
	}
	cmd.AddCommand(newTokenImportCmd())
	cmd.AddCommand(newTokenInfoCmd())
	return cmd
}

func newTokenImportCmd() *cobra.Command {
	var (
		charlesPath string
		harPath     string
		paste       bool
		appleID     string
	)
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a session from a Charles/HAR capture or pasted headers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := importSession(cmd.InOrStdin(), charlesPath, harPath, paste)
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
			fmt.Fprintf(cmd.OutOrStdout(), "saved session to %s\n", path)
			printSession(cmd.OutOrStdout(), sess)
			return nil
		},
	}
	cmd.Flags().StringVar(&charlesPath, "charles", "", "path to a Charles .chlz/.chls session")
	cmd.Flags().StringVar(&harPath, "har", "", "path to a HAR or Charles-JSON export")
	cmd.Flags().BoolVar(&paste, "paste", false, "read raw request headers from stdin")
	cmd.Flags().StringVar(&appleID, "apple-id", "", "account email to embed in iTunesMetadata (optional)")
	return cmd
}

func importSession(stdin io.Reader, charlesPath, harPath string, paste bool) (credential.Session, error) {
	switch {
	case charlesPath != "":
		return charles.ParseDump(charlesPath)
	case harPath != "":
		return charles.ParseHAR(harPath)
	case paste:
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return credential.Session{}, err
		}
		return charles.ParseHeaders(string(raw))
	default:
		return credential.Session{}, fmt.Errorf("%w: specify one of --charles, --har, --paste", errBadInput)
	}
}

func newTokenInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show the active session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession()
			if err != nil {
				return err
			}
			printSession(cmd.OutOrStdout(), sess)
			if err := sess.Valid(); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "validity: INCOMPLETE (%v)\n", err)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "validity: fields OK (run a `versions`/`download` to confirm the token is live)")
			return nil
		},
	}
}

func printSession(w io.Writer, s credential.Session) {
	fmt.Fprintf(w, "dsid:        %s\n", s.DSID)
	fmt.Fprintf(w, "storefront:  %s\n", s.StoreFront)
	fmt.Fprintf(w, "guid:        %s\n", s.GUID)
	fmt.Fprintf(w, "user-agent:  %s\n", s.UserAgent)
	fmt.Fprintf(w, "x-token:     %s\n", redact(s.XToken))
	fmt.Fprintf(w, "cookies:     %d\n", len(s.Cookies))
	fmt.Fprintf(w, "captured:    %s\n", s.CapturedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(w, "source:      %s\n", s.Source)
	if s.AppleID != "" {
		fmt.Fprintf(w, "apple-id:    %s\n", s.AppleID)
	}
}

// redact shows only the length and a short prefix of a secret.
func redact(secret string) string {
	if secret == "" {
		return "(none)"
	}
	n := len(secret)
	if n <= 8 {
		return fmt.Sprintf("(%d bytes)", n)
	}
	return fmt.Sprintf("%s… (%d bytes)", secret[:8], n)
}
