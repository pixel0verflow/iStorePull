// Package cmd wires the istorepull cobra command tree.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/pixel0verflow/istorepull/pkg/credential"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags.
var version = "dev"

// Exit codes (see plan §6).
const (
	exitOK         = 0
	exitTokenDead  = 2 // session expired / invalid
	exitNotServed  = 3 // build no longer served
	exitBadInput   = 4 // bad CLI input
	exitGenericErr = 1
)

// global flags
var (
	flagSession    string
	flagStoreFront string
	flagVerbose    bool
)

// sentinel for CLI input errors so Execute can map the exit code.
var errBadInput = errors.New("bad input")

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "istorepull",
		Short:         "Pull encrypted IPAs from Apple using a borrowed Configurator session",
		Long:          "istorepull replays a Configurator-captured Apple Store session to list app versions and download IPAs. It never logs in.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.PersistentFlags().StringVar(&flagSession, "session", "", "path to session.json (default ~/.istorepull/session.json)")
	root.PersistentFlags().StringVar(&flagStoreFront, "storefront", "", "override X-Apple-Store-Front")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")

	root.AddCommand(newTokenCmd())
	root.AddCommand(newCaptureCmd())
	root.AddCommand(newLookupCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newVersionsCmd())
	root.AddCommand(newDownloadCmd())
	return root
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return exitFor(err)
	}
	return exitOK
}

func exitFor(err error) int {
	switch {
	case errors.Is(err, errBadInput):
		return exitBadInput
	default:
		return exitGenericErr
	}
}

// sessionPath resolves the active session path from the flag or default.
func sessionPath() (string, error) {
	if flagSession != "" {
		return flagSession, nil
	}
	return credential.DefaultPath()
}

// loadSession loads the active session and applies global overrides.
func loadSession() (credential.Session, error) {
	path, err := sessionPath()
	if err != nil {
		return credential.Session{}, err
	}
	s, err := credential.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, fmt.Errorf("no session at %s — run `istorepull token import` first", path)
		}
		return s, err
	}
	if flagStoreFront != "" {
		s.StoreFront = flagStoreFront
	}
	return s, nil
}
