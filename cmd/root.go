package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagProject      string
	flagSubscription string
	flagDebug        bool
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "hac-helper",
	Short: "CCV2 HAC helper — SAML auth and environment management",
	Long: `hac-helper authenticates against the SAP Commerce Cloud portal
using SAML 2.0 SSO and provides CLI access to environment management APIs.`,
}

// Execute is the entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", defaultProject, "project name (as in config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&flagSubscription, "subscription", "s", "", "subscription key (as in config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "dump HTTP responses from SAML flow to stderr")
}
