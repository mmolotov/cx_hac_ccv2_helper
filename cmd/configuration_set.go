package cmd

import (
	"fmt"
	"strings"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// configKV holds a single key=value pair for configuration set.
type configKV struct{ key, value string }

// parseKVPairs parses a slice of "key=value" strings into configKV pairs.
func parseKVPairs(args []string) ([]configKV, error) {
	pairs := make([]configKV, 0, len(args))
	for _, arg := range args {
		idx := strings.IndexByte(arg, '=')
		if idx < 1 {
			return nil, fmt.Errorf("invalid argument %q: expected key=value format", arg)
		}
		pairs = append(pairs, configKV{key: arg[:idx], value: arg[idx+1:]})
	}
	return pairs, nil
}

var (
	propSetEnv        string
	propSetAspects    []string
	propSetAllAspects bool
	propSetHACPath    string
)

var configurationSetCmd = &cobra.Command{
	Use:   "set <key>=<value> [<key>=<value> ...]",
	Short: "Write properties to HAC platform config store",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runConfigurationSet,
}

func init() {
	configurationCmd.AddCommand(configurationSetCmd)
	f := configurationSetCmd.Flags()
	f.StringVarP(&propSetEnv, "environment", "e", defaultEnvironment, "environment code")
	f.StringSliceVarP(&propSetAspects, "aspect", "a", nil, "aspect(s) to target (default: all)")
	f.BoolVar(&propSetAllAspects, "all-aspects", false, "target all configured aspects (default when --aspect is omitted)")
	f.StringVar(&propSetHACPath, "hac-path", defaultHACPath, "path to HAC within the aspect URL")
}

func runConfigurationSet(_ *cobra.Command, args []string) error {
	pairs, err := parseKVPairs(args)
	if err != nil {
		return err
	}
	project, subscription, err := resolveProjectSub(defaultSubscription)
	if err != nil {
		return err
	}
	return runOnHAC(HACRunOpts{
		Project:      project,
		Subscription: subscription,
		Environment:  propSetEnv,
		Aspects:      propSetAspects,
		AllAspects:   propSetAllAspects,
		HACPath:      propSetHACPath,
	}, func(client *hac.Client, _ hac.Target) error {
		return execConfigurationSet(client, pairs)
	})
}

func execConfigurationSet(client *hac.Client, pairs []configKV) error {
	for _, p := range pairs {
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Setting %s...", p.key))
		if err := client.SetConfigProperty(p.key, p.value); err != nil {
			spinner.Fail(fmt.Sprintf("Failed: %v", err))
			return err
		}
		spinner.Success(fmt.Sprintf("%s = %s", p.key, p.value))
	}
	return nil
}
