package cmd

import (
	"fmt"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	cacheClearEnv        string
	cacheClearAspects    []string
	cacheClearAllAspects bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Cache management",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear region cache via HAC",
	Args:  cobra.NoArgs,
	RunE:  runCacheClear,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	f := cacheClearCmd.Flags()
	f.StringVarP(&cacheClearEnv, "environment", "e", defaultEnvironment, "environment code")
	f.StringSliceVarP(&cacheClearAspects, "aspect", "a", nil, "aspect(s) to target (default: all)")
	f.BoolVar(&cacheClearAllAspects, "all-aspects", false, "target all configured aspects")
}

func runCacheClear(_ *cobra.Command, _ []string) error {
	project, subscription, err := resolveProjectSub(defaultSubscription)
	if err != nil {
		return err
	}
	return runOnHAC(HACRunOpts{
		Project:      project,
		Subscription: subscription,
		Environment:  cacheClearEnv,
		Aspects:      cacheClearAspects,
		AllAspects:   cacheClearAllAspects,
		HACPath:      defaultHACPath,
	}, func(client *hac.Client, target hac.Target) error {
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Clearing cache on %s...", target.Tag))
		if err := client.ClearRegionCache(); err != nil {
			spinner.Fail(fmt.Sprintf("Failed: %v", err))
			return err
		}
		spinner.Success("Cache cleared")
		return nil
	})
}
