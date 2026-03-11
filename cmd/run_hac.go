package cmd

import (
	"fmt"
	"time"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/ui"
	"github.com/pterm/pterm"
)

// HACRunOpts configures target resolution for multi-target HAC execution.
type HACRunOpts struct {
	Project      string
	Subscription string
	Environment  string
	Aspects      []string // nil or empty: all aspects (unless AllAspects is redundant)
	AllAspects   bool     // explicitly run on all configured aspects
	HACPath      string   // e.g. "/hac"
	Node         string   // optional: pin requests to a specific cluster node (ROUTE cookie)
}

// runOnHAC resolves HAC targets from config + vault, then calls fn on each target.
// Supports both direct and CCV2 project types.
func runOnHAC(opts HACRunOpts, fn func(*hac.Client, hac.Target) error) error {
	targets, err := resolveHACTargets(opts)
	if err != nil {
		return err
	}
	return hac.ExecuteAll(targets, flagDebug, fn)
}

// newHACClient resolves config + credentials and returns a single authenticated
// HAC client (first resolved target). Used by single-target commands (flexsearch, enumcodes).
func newHACClient(project, subscription, environment, aspect, hacPath string) (*hac.Client, error) {
	targets, err := resolveHACTargets(HACRunOpts{
		Project:      project,
		Subscription: subscription,
		Environment:  environment,
		Aspects:      []string{aspect},
		HACPath:      hacPath,
	})
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets resolved for aspect %q in environment %q", aspect, environment)
	}
	return hac.DialTarget(targets[0], flagDebug)
}

// resolveProjectSub returns (project, subscription) from global flags.
// subscription falls back to defaultSub when --subscription is not provided.
func resolveProjectSub(defaultSub string) (project, subscription string, err error) {
	if flagProject == "" {
		return "", "", fmt.Errorf("--project flag is required")
	}
	subscription = flagSubscription
	if subscription == "" {
		subscription = defaultSub
	}
	return flagProject, subscription, nil
}

// resolveHACTargets is the shared core that loads config + vault and returns
// fully resolved HAC targets for both direct and CCV2 project types.
func resolveHACTargets(opts HACRunOpts) ([]hac.Target, error) {
	projCfg, err := config.LoadProject(opts.Project)
	if err != nil {
		return nil, fmt.Errorf("loading project config: %w", err)
	}
	if projCfg == nil {
		return nil, fmt.Errorf("project %q not found — run 'config setup' first", opts.Project)
	}

	masterPwd, err := ui.PromptPassword("Vault master password")
	if err != nil {
		return nil, err
	}
	store, err := config.LoadCredentials(masterPwd)
	if err != nil {
		return nil, fmt.Errorf("loading credentials: %w", err)
	}

	if projCfg.Type == "direct" {
		hacPwd := ""
		if pc, ok := store.Projects[opts.Project]; ok {
			hacPwd = pc.HACPassword
		}
		if hacPwd == "" {
			hacPwd, err = ui.PromptPassword("HAC password")
			if err != nil {
				return nil, err
			}
		}
		return hac.ResolveTargetsDirect(projCfg, hacPwd), nil
	}

	// CCV2 path.
	sub, ok := projCfg.Subscriptions[opts.Subscription]
	if !ok {
		return nil, fmt.Errorf("subscription %q not found under project %q", opts.Subscription, opts.Project)
	}
	warnIfStale(opts.Project, opts.Subscription, sub.LastUpdated)

	env, ok := sub.Environments[opts.Environment]
	if !ok {
		return nil, fmt.Errorf("environment %q not found in subscription %q", opts.Environment, opts.Subscription)
	}

	login, password := config.ResolveHACCreds(store, opts.Project, opts.Subscription, opts.Environment)
	if login == "" {
		login, err = ui.PromptText("HAC username")
		if err != nil {
			return nil, err
		}
	}
	if password == "" {
		password, err = ui.PromptPassword("HAC password")
		if err != nil {
			return nil, err
		}
	}

	// Empty Aspects slice means "run on all" — same default as explicit --all-aspects.
	allAspects := opts.AllAspects || len(opts.Aspects) == 0
	aspects, err := config.ResolveAspects(env, opts.Aspects, allAspects)
	if err != nil {
		return nil, err
	}
	return hac.ResolveTargetsCCV2(aspects, login, password, opts.Node, opts.HACPath), nil
}

// warnIfStale prints a warning when the project config was last refreshed more than 24h ago.
func warnIfStale(project, subscription string, lastUpdated time.Time) {
	const staleThreshold = 24 * time.Hour
	if !lastUpdated.IsZero() && time.Since(lastUpdated) > staleThreshold {
		pterm.Warning.Printf(
			"Config for %s/%s is %.0f hours old — consider running: config refresh -p %s -s %s\n",
			project, subscription, time.Since(lastUpdated).Hours(), project, subscription,
		)
	}
}
