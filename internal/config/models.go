package config

import "time"

// Config is the top-level YAML configuration structure (aggregate over all projects).
type Config struct {
	Projects map[string]ProjectConfig `yaml:"projects"`
}

// ProjectConfig holds subscriptions for a project. It is the root of a per-project YAML file.
// Type is either "ccv2" (default, uses CCV2 portal) or "direct" (bypasses portal, connects
// directly to a HAC URL stored in HACURL/HACUser).
type ProjectConfig struct {
	Type          string                  `yaml:"type,omitempty"`          // "ccv2" or "direct"
	Subscriptions map[string]Subscription `yaml:"subscriptions,omitempty"` // ccv2 only
	HACURL        string                  `yaml:"hac_url,omitempty"`       // direct only
	HACUser       string                  `yaml:"hac_user,omitempty"`      // direct only
}

// Subscription holds a subscription ID, its environments, and a refresh timestamp.
type Subscription struct {
	SubscriptionID string                 `yaml:"subscription_id"`
	LastUpdated    time.Time              `yaml:"last_updated,omitempty"`
	Environments   map[string]Environment `yaml:"environments"`
}

// Environment is a single CCV2 environment entry.
type Environment struct {
	Code    string            `yaml:"code"`
	Name    string            `yaml:"name"`
	Aspects map[string]Aspect `yaml:"aspects"`
}

// Aspect represents a service endpoint within an environment.
type Aspect struct {
	Code        string   `yaml:"code"`
	Name        string   `yaml:"name"`
	URL         string   `yaml:"url"`
	ContextPath string   `yaml:"context_path,omitempty"`
	Routes      []string `yaml:"routes,omitempty"`
}

// CredentialsStore is the root structure stored encrypted in the vault.
type CredentialsStore struct {
	Projects map[string]ProjectCreds `json:"projects"`
}

// ProjectCreds holds credentials for a project.
// For direct projects: HACPassword is the HAC password.
// For ccv2 projects: Subscriptions holds per-subscription SAP + HAC credentials.
type ProjectCreds struct {
	HACPassword   string              `json:"hac_password,omitempty"`  // direct
	Subscriptions map[string]SubCreds `json:"subscriptions,omitempty"` // ccv2
}

// SubCreds contains all credentials for a subscription.
type SubCreds struct {
	SAPLogin           string              `json:"sap_login"`
	SAPPassword        string              `json:"sap_password"`
	DefaultHACLogin    string              `json:"default_hac_login"`
	DefaultHACPassword string              `json:"default_hac_password"`
	Environments       map[string]EnvCreds `json:"environments,omitempty"`
}

// EnvCreds holds per-environment HAC credential overrides.
type EnvCreds struct {
	HACLogin    string `json:"hac_login"`
	HACPassword string `json:"hac_password"`
}
