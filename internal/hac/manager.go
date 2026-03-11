package hac

import (
	"fmt"
	"strings"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
	"github.com/pterm/pterm"
)

// Target represents a single resolved HAC connection point.
type Target struct {
	Tag      string // display name, e.g. "backoffice" or "backoffice/node-abc"
	URL      string // full HAC base URL (including /hac path)
	Login    string
	Password string
	Route    string // empty means no node pinning
}

// ResolveTargetsDirect builds a single Target for a direct project.
func ResolveTargetsDirect(projCfg *config.ProjectConfig, hacPwd string) []Target {
	user := projCfg.HACUser
	if user == "" {
		user = "admin"
	}
	return []Target{{
		Tag:      "direct",
		URL:      strings.TrimRight(projCfg.HACURL, "/"),
		Login:    user,
		Password: hacPwd,
	}}
}

// ResolveTargetsCCV2 builds the list of Targets for the given aspects, expanding
// each aspect into per-route targets where configured. node overrides config routes
// when non-empty (targets a specific cluster node). hacPath (e.g. "/hac") is
// appended to each aspect URL.
func ResolveTargetsCCV2(
	aspects []config.Aspect,
	login, password string,
	node, hacPath string,
) []Target {
	var targets []Target
	for _, aspect := range aspects {
		hacURL := strings.TrimRight(aspect.URL, "/") + hacPath

		routes := []string{""}
		if node != "" {
			routes = []string{node}
		} else if len(aspect.Routes) > 0 {
			routes = aspect.Routes
		}

		for _, route := range routes {
			tag := aspect.Code
			if route != "" {
				tag = aspect.Code + "/" + route
			}
			targets = append(targets, Target{
				Tag:      tag,
				URL:      hacURL,
				Login:    login,
				Password: password,
				Route:    route,
			})
		}
	}
	return targets
}

// DialTarget creates an authenticated HAC Client for a single Target.
func DialTarget(t Target, debug bool) (*Client, error) {
	client, err := NewClient(t.URL)
	if err != nil {
		return nil, fmt.Errorf("creating HAC client: %w", err)
	}
	client.Debug = debug
	if t.Route != "" {
		client.SetRoute(t.Route)
	}

	label := fmt.Sprintf("Logging in to HAC (%s)", t.URL)
	if t.Route != "" {
		label += " [node: " + t.Route + "]"
	}
	spinner, _ := pterm.DefaultSpinner.Start(label + "...")
	if err := client.Login(t.Login, t.Password); err != nil {
		spinner.Fail("HAC login failed")
		return nil, err
	}
	spinner.Success("HAC authenticated")
	return client, nil
}

// ExecuteAll dials each target and calls fn. Failed logins and fn errors are
// collected; if any targets fail, a combined error is returned.
func ExecuteAll(targets []Target, debug bool, fn func(*Client, Target) error) error {
	multiTarget := len(targets) > 1
	var failed []string

	for _, t := range targets {
		if multiTarget {
			pterm.DefaultSection.Printf("Target: %s\n", t.Tag)
		}
		client, err := DialTarget(t, debug)
		if err != nil {
			pterm.Error.Printf("Login failed (%s): %v\n", t.Tag, err)
			failed = append(failed, t.Tag)
			continue
		}
		if err := fn(client, t); err != nil {
			pterm.Error.Printf("Execution failed (%s): %v\n", t.Tag, err)
			failed = append(failed, t.Tag)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed on: %s", strings.Join(failed, ", "))
	}
	return nil
}
