package cmd

import (
	"fmt"
	"time"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/portal"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var configSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive wizard: authenticate, fetch environments/aspects, save config",
	RunE:  runConfigSetup,
}

func init() {
	configCmd.AddCommand(configSetupCmd)
}

func runConfigSetup(_ *cobra.Command, _ []string) error {
	projectName, err := ui.PromptText("Project name")
	if err != nil {
		return err
	}

	projectType, err := ui.PromptSelect("Project type", []string{
		"ccv2 — SAP Commerce Cloud v2 (portal auth)",
		"direct — direct URL (local / no portal)",
	})
	if err != nil {
		return err
	}

	if projectType == "direct — direct URL (local / no portal)" {
		return setupDirect(projectName)
	}
	return setupCCV2(projectName)
}

// setupDirect configures a direct project (no CCV2 portal): just a HAC URL + credentials.
func setupDirect(projectName string) error {
	masterPwd, err := ui.PromptPassword("Vault master password")
	if err != nil {
		return err
	}
	store, err := config.LoadCredentials(masterPwd)
	if err != nil {
		return fmt.Errorf("loading credentials vault: %w", err)
	}

	existingProj, err := config.LoadProject(projectName)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	if existingProj != nil && existingProj.Type == "direct" {
		ok, err := ui.PromptConfirm(fmt.Sprintf("Overwrite existing direct config for %s?", projectName))
		if err != nil {
			return err
		}
		if !ok {
			pterm.Info.Println("Aborted.")
			return nil
		}
	}

	hacURL, err := ui.PromptTextDefault("HAC base URL", "https://localhost:9002/hac/")
	if err != nil {
		return err
	}
	hacUser, err := ui.PromptText("HAC username (leave blank for 'admin')")
	if err != nil {
		return err
	}
	if hacUser == "" {
		hacUser = "admin"
	}
	hacPwd, err := ui.PromptPassword(fmt.Sprintf("HAC password for %s", hacUser))
	if err != nil {
		return err
	}

	if err := config.SaveProject(projectName, config.ProjectConfig{
		Type:    "direct",
		HACURL:  hacURL,
		HACUser: hacUser,
	}); err != nil {
		return fmt.Errorf("saving project config: %w", err)
	}

	if store.Projects == nil {
		store.Projects = make(map[string]config.ProjectCreds)
	}
	store.Projects[projectName] = config.ProjectCreds{HACPassword: hacPwd}
	if err := config.SaveCredentials(store, masterPwd); err != nil {
		return fmt.Errorf("saving credentials vault: %w", err)
	}

	pterm.Success.Printf("Direct project %q saved — HAC URL: %s, user: %s\n", projectName, hacURL, hacUser)
	return nil
}

// setupCCV2 runs the full CCV2 portal wizard (portal auth, fetch envs/aspects, HAC creds).
func setupCCV2(projectName string) error {
	subAlias, err := ui.PromptText("Subscription alias (e.g. t1, prod)")
	if err != nil {
		return err
	}
	subscriptionID, err := ui.PromptText("CCV2 subscription ID")
	if err != nil {
		return err
	}

	masterPwd, err := ui.PromptPassword("Vault master password")
	if err != nil {
		return err
	}
	store, err := config.LoadCredentials(masterPwd)
	if err != nil {
		return fmt.Errorf("loading credentials vault: %w", err)
	}

	// Extract existing SAP + HAC credentials for this project/subscription (may be empty).
	var existingCreds config.SubCreds
	if projCreds, ok := store.Projects[projectName]; ok {
		if sc, ok := projCreds.Subscriptions[subAlias]; ok {
			existingCreds = sc
		}
	}

	// SAP credentials — reuse from vault or prompt for new.
	sapLogin, sapPwd, err := promptSAPCreds(existingCreds)
	if err != nil {
		return err
	}

	// Check for existing config and confirm overwrite.
	existingProj, err := config.LoadProject(projectName)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	if existingProj != nil {
		if _, ok := existingProj.Subscriptions[subAlias]; ok {
			ok, err := ui.PromptConfirm(
				fmt.Sprintf("Overwrite existing config for %s/%s?", projectName, subAlias),
			)
			if err != nil {
				return err
			}
			if !ok {
				pterm.Info.Println("Aborted.")
				return nil
			}
		}
	}

	// Authenticate via portal.
	portalClient, err := portal.NewPortalClient()
	if err != nil {
		return fmt.Errorf("creating portal client: %w", err)
	}
	portalClient.Debug = flagDebug

	spinner, _ := pterm.DefaultSpinner.Start("Authenticating via SAML SSO...")
	if err := portalClient.Authenticate(subscriptionID, portal.AuthConfig{
		SAPLogin:    sapLogin,
		SAPPassword: sapPwd,
	}); err != nil {
		spinner.Fail("Authentication failed")
		return err
	}
	spinner.Success("Authenticated")

	// Fetch environments.
	spinner, _ = pterm.DefaultSpinner.Start("Fetching environments...")
	envInfos, err := portalClient.GetEnvironments(subscriptionID)
	if err != nil {
		spinner.Fail("Failed to fetch environments")
		return err
	}
	spinner.Success(fmt.Sprintf("Found %d environment(s)", len(envInfos)))

	tableData := pterm.TableData{{"CODE", "NAME", "TYPE", "STATUS"}}
	for _, e := range envInfos {
		tableData = append(tableData, []string{e.Code, e.Name, e.Type, e.Status})
	}
	if err := pterm.DefaultTable.WithHasHeader().WithData(tableData).Render(); err != nil {
		return err
	}

	// For each environment fetch endpoint URLs + services and build aspects.
	environments := make(map[string]config.Environment, len(envInfos))
	for _, envInfo := range envInfos {
		env, err := buildEnvironment(portalClient, subscriptionID, envInfo)
		if err != nil {
			pterm.Warning.Printf("Skipping %s: %v\n", envInfo.Code, err)
			continue
		}
		environments[envInfo.Code] = env
	}

	// HAC credentials.
	defaultHACLogin, defaultHACPassword, perEnvCreds, err := promptHACCreds(existingCreds, envInfos)
	if err != nil {
		return err
	}

	// Save per-project config file.
	if existingProj == nil {
		existingProj = &config.ProjectConfig{
			Subscriptions: make(map[string]config.Subscription),
		}
	}
	existingProj.Type = "ccv2"
	existingProj.Subscriptions[subAlias] = config.Subscription{
		SubscriptionID: subscriptionID,
		LastUpdated:    time.Now().UTC(),
		Environments:   environments,
	}
	if err := config.SaveProject(projectName, *existingProj); err != nil {
		return fmt.Errorf("saving project config: %w", err)
	}

	// Save credentials vault.
	if store.Projects == nil {
		store.Projects = make(map[string]config.ProjectCreds)
	}
	projCreds := store.Projects[projectName]
	if projCreds.Subscriptions == nil {
		projCreds.Subscriptions = make(map[string]config.SubCreds)
	}
	subCreds := config.SubCreds{
		SAPLogin:           sapLogin,
		SAPPassword:        sapPwd,
		DefaultHACLogin:    defaultHACLogin,
		DefaultHACPassword: defaultHACPassword,
	}
	if len(perEnvCreds) > 0 {
		subCreds.Environments = perEnvCreds
	}
	projCreds.Subscriptions[subAlias] = subCreds
	store.Projects[projectName] = projCreds
	if err := config.SaveCredentials(store, masterPwd); err != nil {
		return fmt.Errorf("saving credentials vault: %w", err)
	}

	// Save session.
	if err := portal.SaveSession(portalClient, projectName, subAlias); err != nil {
		pterm.Warning.Printf("Could not save session cache: %v\n", err)
	}

	pterm.Success.Printf("Configuration saved for %s/%s\n", projectName, subAlias)

	summaryData := pterm.TableData{{"ENV", "ASPECTS", "HAC USER"}}
	for _, envInfo := range envInfos {
		hacUser := defaultHACLogin
		if ec, ok := perEnvCreds[envInfo.Code]; ok {
			hacUser = ec.HACLogin
		}
		summaryData = append(summaryData, []string{
			envInfo.Code,
			fmt.Sprintf("%d", len(environments[envInfo.Code].Aspects)),
			hacUser,
		})
	}
	return pterm.DefaultTable.WithHasHeader().WithData(summaryData).Render()
}

// promptSAPCreds offers to reuse existing SAP credentials or prompts for new ones.
func promptSAPCreds(existing config.SubCreds) (login, password string, err error) {
	if existing.SAPLogin != "" {
		reuse, err := ui.PromptConfirm(fmt.Sprintf("Reuse existing SAP credentials (%s)?", existing.SAPLogin))
		if err != nil {
			return "", "", err
		}
		if reuse {
			return existing.SAPLogin, existing.SAPPassword, nil
		}
	}
	login, err = ui.PromptText("SAP login (s-user)")
	if err != nil {
		return "", "", err
	}
	password, err = ui.PromptPassword("SAP password")
	return login, password, err
}

// promptHACCreds prompts for default and per-environment HAC credentials.
func promptHACCreds(existing config.SubCreds, envInfos []portal.EnvironmentInfo) (
	defaultLogin, defaultPassword string, perEnv map[string]config.EnvCreds, err error,
) {
	perEnv = make(map[string]config.EnvCreds)

	if existing.DefaultHACLogin != "" {
		reuse, err := ui.PromptConfirm(fmt.Sprintf("Reuse existing HAC credentials (%s)?", existing.DefaultHACLogin))
		if err != nil {
			return "", "", nil, err
		}
		if reuse {
			if existing.Environments != nil {
				perEnv = existing.Environments
			}
			return existing.DefaultHACLogin, existing.DefaultHACPassword, perEnv, nil
		}
	}

	sameForAll, err := ui.PromptConfirm("Use the same HAC credentials for all environments?")
	if err != nil {
		return "", "", nil, err
	}
	if sameForAll {
		defaultLogin, err = ui.PromptText("HAC username (default for all)")
		if err != nil {
			return "", "", nil, err
		}
		defaultPassword, err = ui.PromptPassword("HAC password (default for all)")
		return defaultLogin, defaultPassword, perEnv, err
	}

	for _, envInfo := range envInfos {
		login, err := ui.PromptText(fmt.Sprintf("HAC username for %s (Enter to use default)", envInfo.Code))
		if err != nil {
			return "", "", nil, err
		}
		if login != "" {
			pwd, err := ui.PromptPassword(fmt.Sprintf("HAC password for %s", envInfo.Code))
			if err != nil {
				return "", "", nil, err
			}
			perEnv[envInfo.Code] = config.EnvCreds{HACLogin: login, HACPassword: pwd}
		}
	}
	defaultLogin, err = ui.PromptText("Default HAC username (fallback)")
	if err != nil {
		return "", "", nil, err
	}
	defaultPassword, err = ui.PromptPassword("Default HAC password")
	return defaultLogin, defaultPassword, perEnv, err
}

// buildEnvironment fetches endpoints + services for one environment and builds its Aspect map.
func buildEnvironment(client *portal.PortalClient, subscriptionID string, envInfo portal.EnvironmentInfo) (config.Environment, error) {
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Fetching endpoint URLs for %s...", envInfo.Code))
	envDetail, err := client.GetEnvironment(subscriptionID, envInfo.Code)
	if err != nil {
		spinner.Warning(fmt.Sprintf("no environment detail: %v", err))
		return config.Environment{}, err
	}
	endpointURLs := make(map[string]string, len(envDetail.Endpoints))
	for _, ep := range envDetail.Endpoints {
		if ep.DomainName != "" {
			endpointURLs[ep.Code] = "https://" + ep.DomainName
		}
	}
	spinner.Success(fmt.Sprintf("%s: %d endpoint(s)", envInfo.Code, len(endpointURLs)))

	spinner, _ = pterm.DefaultSpinner.Start(fmt.Sprintf("Fetching services for %s...", envInfo.Code))
	services, err := client.GetServices(subscriptionID, envInfo.Code)
	if err != nil {
		spinner.Warning(fmt.Sprintf("no services: %v", err))
		return config.Environment{}, err
	}
	spinner.Success(fmt.Sprintf("%s: %d service(s)", envInfo.Code, len(services)))

	aspects := make(map[string]config.Aspect, len(services))
	for _, svc := range services {
		if !config.RelevantAspects[svc.Code] {
			continue
		}
		defaultURL := ""
		if epCode, ok := config.ServiceToEndpoint[svc.Code]; ok {
			defaultURL = endpointURLs[epCode]
		}
		chosenURL, err := promptAspectURL(envInfo.Code, svc.Code, defaultURL)
		if err != nil {
			return config.Environment{}, err
		}
		aspects[svc.Code] = config.Aspect{
			Code:        svc.Code,
			Name:        svc.Name,
			URL:         chosenURL,
			ContextPath: svc.ContextPath,
			Routes:      buildRoutes(svc),
		}
	}

	return config.Environment{Code: envInfo.Code, Name: envInfo.Name, Aspects: aspects}, nil
}

// promptAspectURL asks the user to confirm or override the API-provided URL for an aspect.
func promptAspectURL(envCode, aspectCode, defaultURL string) (string, error) {
	if defaultURL != "" {
		choice, err := ui.PromptSelect(
			fmt.Sprintf("URL for aspect %s in %s", aspectCode, envCode),
			[]string{"Use API URL: " + defaultURL, "Enter custom URL"},
		)
		if err != nil {
			return "", err
		}
		if choice != "Enter custom URL" {
			return defaultURL, nil
		}
		return ui.PromptText(fmt.Sprintf("Custom URL for %s/%s", envCode, aspectCode))
	}
	return ui.PromptText(fmt.Sprintf("URL for aspect %s in %s", aspectCode, envCode))
}

// buildRoutes returns the READY replica names for an aspect (used as ROUTE cookie values).
func buildRoutes(svc portal.ServiceInfo) []string {
	routes := make([]string, 0, len(svc.Replicas))
	for _, r := range svc.Replicas {
		if r.Ready {
			routes = append(routes, r.Name)
		}
	}
	return routes
}
