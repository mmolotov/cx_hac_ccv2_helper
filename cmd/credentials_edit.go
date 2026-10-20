package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var credsEditEnv string

var credsEditCmd = &cobra.Command{
	Use:   "edit <project>",
	Short: "Edit SAP/HAC credentials of a project in the vault",
	Long: `Interactively edit stored credentials and optionally verify the HAC login.

direct projects: HAC username (project config) and HAC password (vault).
ccv2 projects:   SAP login/password, default HAC login/password, or a per-environment
                 HAC override. Use -s to pick the subscription and -e to jump straight
                 to the HAC credentials of one environment.

At password prompts, press Enter to keep the current password.`,
	Example: `  hac-helper credentials edit myproj
  hac-helper credentials edit myproj -s t1 -e p1`,
	Args: cobra.ExactArgs(1),
	RunE: runCredsEdit,
}

func init() {
	credsCmd.AddCommand(credsEditCmd)
	credsEditCmd.Flags().StringVarP(&credsEditEnv, "environment", "e", "", "edit HAC credentials of this environment (ccv2)")
}

func runCredsEdit(_ *cobra.Command, args []string) error {
	project := args[0]
	projCfg, err := config.LoadProject(project)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project %q not found — run 'config setup' first", project)
	}

	masterPwd, err := ui.PromptPassword("Vault master password")
	if err != nil {
		return err
	}
	store, err := config.LoadCredentials(masterPwd)
	if err != nil {
		return fmt.Errorf("loading credentials vault: %w", err)
	}
	if store.Projects == nil {
		store.Projects = make(map[string]config.ProjectCreds)
	}

	if projCfg.Type == "direct" {
		return editDirectCreds(project, projCfg, store, masterPwd)
	}
	return editCCV2Creds(project, projCfg, store, masterPwd)
}

func editDirectCreds(project string, projCfg *config.ProjectConfig, store *config.CredentialsStore, masterPwd string) error {
	user, err := ui.PromptTextDefault("HAC username", projCfg.HACUser)
	if err != nil {
		return err
	}
	pc := store.Projects[project]
	pwd, err := promptKeepPassword(fmt.Sprintf("HAC password for %s", user), pc.HACPassword)
	if err != nil {
		return err
	}

	projCfg.HACUser = strings.TrimSpace(user)
	if err := config.SaveProject(project, *projCfg); err != nil {
		return fmt.Errorf("saving project config: %w", err)
	}
	pc.HACPassword = pwd
	store.Projects[project] = pc
	if err := config.SaveCredentials(store, masterPwd); err != nil {
		return fmt.Errorf("saving credentials vault: %w", err)
	}
	pterm.Success.Printf("Credentials for %q updated.\n", project)

	return offerHACCheck(func() (hac.Target, error) {
		return hac.ResolveTargetsDirect(projCfg, pwd)[0], nil
	})
}

func editCCV2Creds(project string, projCfg *config.ProjectConfig, store *config.CredentialsStore, masterPwd string) error {
	subAlias, err := pickSubscription(project, projCfg)
	if err != nil {
		return err
	}
	sub := projCfg.Subscriptions[subAlias]

	pc := store.Projects[project]
	if pc.Subscriptions == nil {
		pc.Subscriptions = make(map[string]config.SubCreds)
	}
	sc := pc.Subscriptions[subAlias]

	envCode := credsEditEnv
	editDefault := false
	if envCode == "" {
		envCode, editDefault, err = pickCCV2CredsItem(sc, sub)
		if err != nil {
			return err
		}
	} else if _, ok := sub.Environments[envCode]; !ok {
		return fmt.Errorf("environment %q not found in subscription %q", envCode, subAlias)
	}

	switch {
	case envCode == "" && !editDefault:
		login, err := ui.PromptTextDefault("SAP login (s-user)", sc.SAPLogin)
		if err != nil {
			return err
		}
		pwd, err := promptKeepPassword("SAP password", sc.SAPPassword)
		if err != nil {
			return err
		}
		sc.SAPLogin, sc.SAPPassword = strings.TrimSpace(login), pwd
	case editDefault:
		login, err := ui.PromptTextDefault("Default HAC username", sc.DefaultHACLogin)
		if err != nil {
			return err
		}
		pwd, err := promptKeepPassword("Default HAC password", sc.DefaultHACPassword)
		if err != nil {
			return err
		}
		sc.DefaultHACLogin, sc.DefaultHACPassword = strings.TrimSpace(login), pwd
	default:
		curLogin, _ := effectiveHACCreds(sc, envCode)
		login, err := ui.PromptTextDefault(
			fmt.Sprintf("HAC username for %s (clear to use the default %q)", envCode, sc.DefaultHACLogin), curLogin)
		if err != nil {
			return err
		}
		pwd := ""
		if strings.TrimSpace(login) != "" {
			_, curPwd := effectiveHACCreds(sc, envCode)
			if pwd, err = promptKeepPassword(fmt.Sprintf("HAC password for %s", envCode), curPwd); err != nil {
				return err
			}
		}
		setEnvHACCreds(&sc, envCode, login, pwd)
	}

	pc.Subscriptions[subAlias] = sc
	store.Projects[project] = pc
	if err := config.SaveCredentials(store, masterPwd); err != nil {
		return fmt.Errorf("saving credentials vault: %w", err)
	}
	pterm.Success.Printf("Credentials for %s/%s updated.\n", project, subAlias)

	if envCode == "" && !editDefault {
		return nil // SAP creds are verified by the next 'auth login'
	}
	return offerHACCheck(func() (hac.Target, error) {
		checkEnv := envCode
		if checkEnv == "" {
			if checkEnv, err = pickEnvironment(sub); err != nil {
				return hac.Target{}, err
			}
		}
		login, pwd := effectiveHACCreds(sc, checkEnv)
		aspects, err := config.ResolveAspects(sub.Environments[checkEnv], []string{defaultAspect}, false)
		if err != nil {
			return hac.Target{}, err
		}
		targets := hac.ResolveTargetsCCV2(aspects, login, pwd, "", defaultHACPath)
		if len(targets) == 0 {
			return hac.Target{}, fmt.Errorf("no HAC target for %s/%s", checkEnv, defaultAspect)
		}
		return targets[0], nil
	})
}

// effectiveHACCreds returns the HAC login/password used for env: the per-env
// override if present, otherwise the subscription default.
func effectiveHACCreds(sc config.SubCreds, env string) (login, password string) {
	if ec, ok := sc.Environments[env]; ok {
		return ec.HACLogin, ec.HACPassword
	}
	return sc.DefaultHACLogin, sc.DefaultHACPassword
}

// setEnvHACCreds updates the HAC override for env. An empty login removes the
// override so env falls back to the default credentials; an empty password
// keeps the currently effective password.
func setEnvHACCreds(sc *config.SubCreds, env, login, password string) {
	login = strings.TrimSpace(login)
	if login == "" {
		delete(sc.Environments, env)
		return
	}
	if password == "" {
		_, password = effectiveHACCreds(*sc, env)
	}
	if sc.Environments == nil {
		sc.Environments = make(map[string]config.EnvCreds)
	}
	sc.Environments[env] = config.EnvCreds{HACLogin: login, HACPassword: password}
}

// promptKeepPassword prompts for a password; empty input keeps current.
func promptKeepPassword(label, current string) (string, error) {
	if current != "" {
		label += " (Enter to keep current)"
	}
	pwd, err := ui.PromptPassword(label)
	if err != nil {
		return "", err
	}
	if pwd == "" {
		return current, nil
	}
	return pwd, nil
}

func pickSubscription(project string, projCfg *config.ProjectConfig) (string, error) {
	if flagSubscription != "" {
		if _, ok := projCfg.Subscriptions[flagSubscription]; !ok {
			return "", fmt.Errorf("subscription %q not found under project %q", flagSubscription, project)
		}
		return flagSubscription, nil
	}
	subs := make([]string, 0, len(projCfg.Subscriptions))
	for alias := range projCfg.Subscriptions {
		subs = append(subs, alias)
	}
	sort.Strings(subs)
	switch len(subs) {
	case 0:
		return "", fmt.Errorf("project %q has no subscriptions", project)
	case 1:
		return subs[0], nil
	}
	return ui.PromptSelect("Subscription", subs)
}

func pickEnvironment(sub config.Subscription) (string, error) {
	return ui.PromptSelect("Environment to check", sortedEnvCodes(sub))
}

func sortedEnvCodes(sub config.Subscription) []string {
	codes := make([]string, 0, len(sub.Environments))
	for code := range sub.Environments {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// pickCCV2CredsItem asks what to edit. It returns the env code for a per-env
// HAC override, editDefault=true for the default HAC creds, or ("", false) for SAP creds.
func pickCCV2CredsItem(sc config.SubCreds, sub config.Subscription) (envCode string, editDefault bool, err error) {
	sapItem := fmt.Sprintf("SAP login/password (%s)", orNotSet(sc.SAPLogin))
	defItem := fmt.Sprintf("Default HAC login/password (%s)", orNotSet(sc.DefaultHACLogin))
	items := []string{sapItem, defItem}
	envByItem := map[string]string{}
	for _, code := range sortedEnvCodes(sub) {
		item := fmt.Sprintf("HAC for %s: default (%s)", code, orNotSet(sc.DefaultHACLogin))
		if ec, ok := sc.Environments[code]; ok {
			item = fmt.Sprintf("HAC for %s: override (%s)", code, orNotSet(ec.HACLogin))
		}
		items = append(items, item)
		envByItem[item] = code
	}

	choice, err := ui.PromptSelect("What to edit", items)
	if err != nil {
		return "", false, err
	}
	switch choice {
	case sapItem:
		return "", false, nil
	case defItem:
		return "", true, nil
	}
	return envByItem[choice], false, nil
}

func orNotSet(s string) string {
	if s == "" {
		return "not set"
	}
	return s
}

// offerHACCheck optionally logs in to HAC with the updated credentials.
func offerHACCheck(target func() (hac.Target, error)) error {
	ok, err := ui.PromptConfirm("Verify HAC login now?")
	if err != nil || !ok {
		return err
	}
	t, err := target()
	if err != nil {
		return err
	}
	_, err = hac.DialTarget(t, flagDebug)
	return err
}
