package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/portal"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage portal authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate against the CCV2 portal via SAML SSO",
	RunE:  runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the cached session for a project/subscription",
	RunE:  runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status for all configured sessions",
	RunE:  runAuthStatus,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
}

// ---------------------------------------------------------------------------
// login
// ---------------------------------------------------------------------------

func runAuthLogin(cmd *cobra.Command, _ []string) error {
	project, subscription, err := requireProjectSub()
	if err != nil {
		return err
	}

	projCfg, err := config.LoadProject(project)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project %q not found in config — run 'config setup' first", project)
	}
	if projCfg.Type == "direct" {
		return fmt.Errorf("project %q is a direct project — portal auth is not applicable", project)
	}
	sub, ok := projCfg.Subscriptions[subscription]
	if !ok {
		return fmt.Errorf("subscription %q not found under project %q", subscription, project)
	}

	// Check for valid cached session
	sess, err := portal.LoadSession(project, subscription)
	if err != nil {
		return fmt.Errorf("loading session cache: %w", err)
	}
	if sess != nil {
		pterm.Success.Printf("Already authenticated (session valid until %s)\n",
			sess.ExpiresAt.Format(time.RFC3339))
		return nil
	}

	// Load credentials from vault
	masterPwd, err := ui.PromptPassword("Vault master password")
	if err != nil {
		return err
	}

	store, err := config.LoadCredentials(masterPwd)
	if err != nil {
		return fmt.Errorf("loading credentials vault: %w", err)
	}

	var sapLogin, sapPwd string
	if projCreds, ok := store.Projects[project]; ok {
		if subCreds, ok := projCreds.Subscriptions[subscription]; ok {
			sapLogin = subCreds.SAPLogin
			sapPwd = subCreds.SAPPassword
		}
	}

	if sapLogin == "" {
		sapLogin, err = ui.PromptText("SAP login (s-user)")
		if err != nil {
			return err
		}
	}
	if sapPwd == "" {
		sapPwd, err = ui.PromptPassword("SAP password")
		if err != nil {
			return err
		}
	}

	client, err := portal.NewPortalClient()
	if err != nil {
		return fmt.Errorf("creating portal client: %w", err)
	}
	client.Debug = flagDebug

	spinner, _ := pterm.DefaultSpinner.Start("Authenticating via SAML SSO...")

	if err := client.Authenticate(sub.SubscriptionID, portal.AuthConfig{
		SAPLogin:    sapLogin,
		SAPPassword: sapPwd,
	}); err != nil {
		spinner.Fail("Authentication failed")
		return err
	}

	spinner.Success("Authentication successful")

	if err := portal.SaveSession(client, project, subscription); err != nil {
		pterm.Warning.Printf("Could not save session cache: %v\n", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// logout
// ---------------------------------------------------------------------------

func runAuthLogout(cmd *cobra.Command, _ []string) error {
	project, subscription, err := requireProjectSub()
	if err != nil {
		return err
	}

	if err := portal.DeleteSession(project, subscription); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	pterm.Success.Printf("Session for %s/%s removed.\n", project, subscription)
	return nil
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

func runAuthStatus(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tSUBSCRIPTION\tLOGGED IN\tEXPIRES AT")

	for proj, projCfg := range cfg.Projects {
		for sub := range projCfg.Subscriptions {
			loggedIn := "no"
			expiresAt := "-"

			sess, err := portal.LoadSession(proj, sub)
			if err == nil && sess != nil {
				loggedIn = "yes"
				expiresAt = sess.ExpiresAt.Format(time.RFC3339)
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", proj, sub, loggedIn, expiresAt)
		}
	}

	return w.Flush()
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func requireProjectSub() (project, subscription string, err error) {
	if flagProject == "" {
		return "", "", fmt.Errorf("--project flag is required")
	}
	if flagSubscription == "" {
		return "", "", fmt.Errorf("--subscription flag is required")
	}
	return flagProject, flagSubscription, nil
}
