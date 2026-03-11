package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/portal"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var configRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Re-fetch environments and update aspect routes without changing configured URLs",
	RunE:  runConfigRefresh,
}

func init() {
	configCmd.AddCommand(configRefreshCmd)
}

func runConfigRefresh(_ *cobra.Command, _ []string) error {
	project, subscription, err := requireProjectSub()
	if err != nil {
		return err
	}

	projCfg, err := config.LoadProject(project)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project %q not found — run 'config setup' first", project)
	}
	if projCfg.Type == "direct" {
		return fmt.Errorf("project %q is a direct project — config refresh is not applicable", project)
	}
	sub, ok := projCfg.Subscriptions[subscription]
	if !ok {
		return fmt.Errorf("subscription %q not found under project %q — run 'config setup' first", subscription, project)
	}

	// Authenticate: prefer cached session, fall back to full SAML auth.
	client, err := portal.NewPortalClient()
	if err != nil {
		return fmt.Errorf("creating portal client: %w", err)
	}
	client.Debug = flagDebug

	sess, err := portal.LoadSession(project, subscription)
	if err != nil {
		return fmt.Errorf("loading session cache: %w", err)
	}
	if sess != nil {
		if err := portal.RestoreSession(client, sess); err != nil {
			return fmt.Errorf("restoring session: %w", err)
		}
		pterm.Info.Println("Using cached portal session.")
	} else {
		if err := refreshDoAuth(client, project, subscription, sub.SubscriptionID); err != nil {
			return err
		}
		if err := portal.SaveSession(client, project, subscription); err != nil {
			pterm.Warning.Printf("Could not save session cache: %v\n", err)
		}
	}

	// Re-fetch routes for each environment.
	// On session expiry, re-authenticate once and retry.
	sessionRefreshed := false
	updatedCount := 0
	for envCode, env := range sub.Environments {
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Refreshing services for %s...", envCode))
		services, err := client.GetServices(sub.SubscriptionID, envCode)
		if errors.Is(err, portal.ErrSessionExpired) && !sessionRefreshed {
			spinner.Warning("Session expired, re-authenticating...")
			sessionRefreshed = true
			portal.DeleteSession(project, subscription) //nolint:errcheck
			if authErr := refreshDoAuth(client, project, subscription, sub.SubscriptionID); authErr != nil {
				return authErr
			}
			if err := portal.SaveSession(client, project, subscription); err != nil {
				pterm.Warning.Printf("Could not save session cache: %v\n", err)
			}
			services, err = client.GetServices(sub.SubscriptionID, envCode)
		}
		if err != nil {
			spinner.Warning(fmt.Sprintf("Skipping %s: %v", envCode, err))
			continue
		}
		spinner.Success(fmt.Sprintf("%s: %d service(s)", envCode, len(services)))

		for _, svc := range services {
			if !config.RelevantAspects[svc.Code] {
				continue
			}
			existing, exists := env.Aspects[svc.Code]
			if !exists {
				continue // only update known aspects; don't add new ones during refresh
			}
			existing.Routes = buildRoutes(svc)
			env.Aspects[svc.Code] = existing
			updatedCount++
		}
		sub.Environments[envCode] = env
	}

	sub.LastUpdated = time.Now().UTC()
	projCfg.Subscriptions[subscription] = sub
	if err := config.SaveProject(project, *projCfg); err != nil {
		return fmt.Errorf("saving project config: %w", err)
	}

	pterm.Success.Printf("Refreshed %s/%s — updated routes for %d aspect(s)\n", project, subscription, updatedCount)
	return nil
}

// refreshDoAuth loads SAP credentials from the vault and runs SAML authentication.
// Used for initial auth and for re-auth after session expiry.
func refreshDoAuth(client *portal.PortalClient, project, subscription, subscriptionID string) error {
	masterPwd, err := ui.PromptPassword("Vault master password")
	if err != nil {
		return err
	}
	store, err := config.LoadCredentials(masterPwd)
	if err != nil {
		return fmt.Errorf("loading credentials vault: %w", err)
	}

	var sapLogin, sapPwd string
	if pc, ok := store.Projects[project]; ok {
		if sc, ok := pc.Subscriptions[subscription]; ok {
			sapLogin = sc.SAPLogin
			sapPwd = sc.SAPPassword
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

	spinner, _ := pterm.DefaultSpinner.Start("Authenticating via SAML SSO...")
	if err := client.Authenticate(subscriptionID, portal.AuthConfig{
		SAPLogin:    sapLogin,
		SAPPassword: sapPwd,
	}); err != nil {
		spinner.Fail("Authentication failed")
		return err
	}
	spinner.Success("Authenticated")
	return nil
}
