package cmd

import (
	"fmt"
	"strings"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	propGetEnv     string
	propGetAspects []string
)

var configurationGetCmd = &cobra.Command{
	Use:   "get <key> [<key> ...]",
	Short: "Read SAP configuration properties via HAC Groovy console",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runConfigurationGet,
}

func init() {
	configurationCmd.AddCommand(configurationGetCmd)
	f := configurationGetCmd.Flags()
	f.StringVarP(&propGetEnv, "environment", "e", defaultEnvironment, "environment code")
	f.StringSliceVarP(&propGetAspects, "aspect", "a", nil, "aspect(s) to target (default: all)")
}

func runConfigurationGet(_ *cobra.Command, args []string) error {
	project, subscription, err := resolveProjectSub(defaultSubscription)
	if err != nil {
		return err
	}
	return runOnHAC(HACRunOpts{
		Project:      project,
		Subscription: subscription,
		Environment:  propGetEnv,
		Aspects:      propGetAspects,
		HACPath:      defaultHACPath,
	}, func(client *hac.Client, _ hac.Target) error {
		return execConfigurationGet(client, args)
	})
}

func execConfigurationGet(client *hac.Client, keys []string) error {
	for _, key := range keys {
		script := fmt.Sprintf("configurationService.getConfiguration().getString('%s')", key)
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Reading %s...", key))
		result, err := client.ExecuteScript(hac.ScriptRequest{Script: script})
		if err != nil {
			spinner.Fail(fmt.Sprintf("Request failed: %v", err))
			return err
		}
		if result.HasError() {
			spinner.Fail("Script error")
			msg := strings.SplitN(result.StacktraceText, "\n", 2)[0]
			return fmt.Errorf("%s", msg)
		}
		spinner.Success(key)
		val := result.ExecutionResult
		if val == "" || val == "null" {
			pterm.Warning.Printf("%s = (not set)\n", key)
		} else {
			fmt.Printf("%s = %s\n", key, val)
		}
	}
	return nil
}
