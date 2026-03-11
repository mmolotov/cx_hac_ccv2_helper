package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/spf13/cobra"
)

var (
	uptimeEnv        string
	uptimeAspects    []string
	uptimeAllAspects bool
	uptimeFormat     string
)

const uptimeScript = `java.lang.management.ManagementFactory.getRuntimeMXBean().getStartTime()`

var uptimeCmd = &cobra.Command{
	Use:   "uptime",
	Short: "Show node start time and uptime via HAC Groovy console",
	Example: `  hac-helper uptime
  hac-helper uptime --format timestamp
  hac-helper uptime -a backoffice -a api`,
	Args: cobra.NoArgs,
	RunE: runUptime,
}

func init() {
	rootCmd.AddCommand(uptimeCmd)
	f := uptimeCmd.Flags()
	f.StringVarP(&uptimeEnv, "environment", "e", defaultEnvironment, "environment code")
	f.StringSliceVarP(&uptimeAspects, "aspect", "a", nil, "aspect(s) to target (default: all)")
	f.BoolVar(&uptimeAllAspects, "all-aspects", false, "target all configured aspects")
	f.StringVar(&uptimeFormat, "format", "datetime", "output format: datetime (ISO) or timestamp (ms)")
}

func runUptime(_ *cobra.Command, _ []string) error {
	project, subscription, err := resolveProjectSub(defaultSubscription)
	if err != nil {
		return err
	}
	return runOnHAC(HACRunOpts{
		Project:      project,
		Subscription: subscription,
		Environment:  uptimeEnv,
		Aspects:      uptimeAspects,
		AllAspects:   uptimeAllAspects,
		HACPath:      defaultHACPath,
	}, func(client *hac.Client, target hac.Target) error {
		return execUptime(client, target, uptimeFormat)
	})
}

func execUptime(client *hac.Client, target hac.Target, format string) error {
	result, err := client.ExecuteScript(hac.ScriptRequest{Script: uptimeScript})
	if err != nil {
		return err
	}
	if result.HasError() {
		msg := strings.SplitN(result.StacktraceText, "\n", 2)[0]
		return fmt.Errorf("%s", msg)
	}

	raw := strings.TrimSpace(result.ExecutionResult)
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("unexpected start time value %q: %w", raw, err)
	}

	startTime := time.UnixMilli(ms)
	uptime := time.Since(startTime).Truncate(time.Second)

	switch format {
	case "timestamp":
		fmt.Printf("[%s] started=%d  uptime=%s\n", target.Tag, ms, formatDuration(uptime))
	default:
		fmt.Printf("[%s] started=%s  uptime=%s\n", target.Tag, startTime.Format(time.RFC3339), formatDuration(uptime))
	}
	return nil
}

// formatDuration formats a duration as "Xd Xh Xm Xs".
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%ds", seconds))
	return strings.Join(parts, " ")
}
