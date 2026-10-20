package cmd

import (
	"fmt"
	"strings"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var validLogLevels = map[string]bool{
	"TRACE": true,
	"DEBUG": true,
	"INFO":  true,
	"WARN":  true,
	"ERROR": true,
	"FATAL": true,
	"OFF":   true,
}

var (
	logLevelEnv        string
	logLevelAspects    []string
	logLevelAllAspects bool
)

var logLevelCmd = &cobra.Command{
	Use:   "loglevel <logger>=<LEVEL> [<logger>=<LEVEL> ...]",
	Short: "Change Log4j logger levels via HAC",
	Example: `  hac loglevel de.hybris.platform.order=DEBUG
  hac loglevel de.hybris.platform.order=DEBUG com.example.MyService=WARN -a backoffice`,
	Args: cobra.MinimumNArgs(1),
	RunE: runLogLevel,
}

func init() {
	rootCmd.AddCommand(logLevelCmd)
	f := logLevelCmd.Flags()
	f.StringVarP(&logLevelEnv, "environment", "e", defaultEnvironment, "environment code")
	f.StringSliceVarP(&logLevelAspects, "aspect", "a", nil, "aspect(s) to target (default: all)")
	f.BoolVar(&logLevelAllAspects, "all-aspects", false, "target all configured aspects")
}

func runLogLevel(_ *cobra.Command, args []string) error {
	pairs, err := parseKVPairs(args)
	if err != nil {
		return err
	}
	for _, p := range pairs {
		upper := strings.ToUpper(p.value)
		if !validLogLevels[upper] {
			return fmt.Errorf("invalid log level %q for %q; valid levels: TRACE DEBUG INFO WARN ERROR FATAL OFF", p.value, p.key)
		}
		p.value = upper
	}

	project, subscription, err := resolveProjectSub(defaultSubscription)
	if err != nil {
		return err
	}
	return runOnHAC(HACRunOpts{
		Project:      project,
		Subscription: subscription,
		Environment:  logLevelEnv,
		Aspects:      logLevelAspects,
		AllAspects:   logLevelAllAspects,
		HACPath:      defaultHACPath,
	}, func(client *hac.Client, _ hac.Target) error {
		return execLogLevel(client, pairs)
	})
}

func execLogLevel(client *hac.Client, pairs []configKV) error {
	for _, p := range pairs {
		level := strings.ToUpper(p.value)
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Setting %s → %s...", p.key, level))
		if err := client.ChangeLogLevel(p.key, level); err != nil {
			spinner.Fail(fmt.Sprintf("Failed: %v", err))
			return err
		}
		spinner.Success(fmt.Sprintf("%s → %s", p.key, level))
	}
	return nil
}
