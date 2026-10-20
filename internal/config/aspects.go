package config

import (
	"fmt"
	"sort"
)

// AspectAliases maps short user-friendly names to one or more CCV2 service codes.
var AspectAliases = map[string][]string{
	"api":        {"hcs_platform_api"},
	"backoffice": {"hcs_platform_backoffice"},
	"processing": {"hcs_platform_processing", "hcs_platform_backgroundProcessing"},
	"storefront": {"hcs_platform_storefront", "hcs_platform_accstorefront"},
}

// RelevantAspects is the set of CCV2 service codes that expose a HAC endpoint.
var RelevantAspects = map[string]bool{
	"hcs_platform_api":                  true,
	"hcs_platform_backoffice":           true,
	"hcs_platform_processing":           true,
	"hcs_platform_backgroundProcessing": true,
	"hcs_platform_storefront":           true,
	"hcs_platform_accstorefront":        true,
}

// ServiceToEndpoint maps a CCV2 service code to the endpoint code in the portal API response.
var ServiceToEndpoint = map[string]string{
	"hcs_platform_api":                  "api",
	"hcs_platform_backoffice":           "backoffice",
	"hcs_platform_backgroundProcessing": "processing",
	"hcs_platform_processing":           "processing",
	"hcs_platform_storefront":           "storefront",
	"hcs_platform_accstorefront":        "storefront",
}

// ResolveAspectCodes expands a short alias to one or more full CCV2 service codes.
// Unknown aliases are returned unchanged (full codes pass through).
func ResolveAspectCodes(alias string) []string {
	if codes, ok := AspectAliases[alias]; ok {
		return codes
	}
	return []string{alias}
}

// ResolveHACCreds returns the HAC login/password for a project/subscription/environment
// from the credentials store, applying the per-env → default fallback.
func ResolveHACCreds(store *CredentialsStore, project, subscription, environment string) (login, password string) {
	if pc, ok := store.Projects[project]; ok {
		if sc, ok := pc.Subscriptions[subscription]; ok {
			login = sc.DefaultHACLogin
			password = sc.DefaultHACPassword
			if ec, ok := sc.Environments[environment]; ok {
				login = ec.HACLogin
				password = ec.HACPassword
			}
		}
	}
	return
}

// ResolveAspects returns the list of configured Aspect objects for the given environment
// based on user-supplied alias inputs (or all aspects when allAspects is true).
func ResolveAspects(env Environment, inputs []string, allAspects bool) ([]Aspect, error) {
	if allAspects {
		if len(env.Aspects) == 0 {
			return nil, fmt.Errorf("no aspects configured for environment %q", env.Code)
		}
		aspects := make([]Aspect, 0, len(env.Aspects))
		for _, a := range env.Aspects {
			aspects = append(aspects, a)
		}
		sort.Slice(aspects, func(i, j int) bool { return aspects[i].Code < aspects[j].Code })
		return aspects, nil
	}

	seen := map[string]bool{}
	var aspects []Aspect
	for _, input := range inputs {
		found := false
		for _, code := range ResolveAspectCodes(input) {
			if seen[code] {
				found = true // already selected via another alias or a duplicate input
				continue
			}
			a, ok := env.Aspects[code]
			if !ok {
				continue // alias may cover codes not present in this environment
			}
			seen[code] = true
			aspects = append(aspects, a)
			found = true
		}
		if !found {
			return nil, fmt.Errorf("aspect %q not found in environment %q", input, env.Code)
		}
	}
	return aspects, nil
}
