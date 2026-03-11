package cmd

import (
	"fmt"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	cronJobsEnvironment string
	cronJobsAspect      string
	cronJobsHACPath     string
	cronJobsCronJob     string
	cronJobsJob         string
	cronJobsBean        string
	cronJobsLike        bool
	cronJobsMaxCount    int
	cronJobsFormat      string
)

const cronJobsQueryTemplate = `SELECT {j.code} as job, {cj.code} as cron_job, {cj.pk} as pk, {status.code} as status,` +
	` {result.code} as result, {cj.startTime} as start_time, {cj.endTime} as end_time,` +
	` datediff(second, {cj.startTime}, {cj.endTime}) as duration_sec` +
	` FROM {job as j` +
	` JOIN cronjob as cj ON {cj.job}={j.pk}` +
	` LEFT JOIN servicelayerjob as sj ON {j.pk}={sj.pk}` +
	` LEFT JOIN CronJobStatus as status ON {cj.status}={status.pk}` +
	` LEFT JOIN CronJobResult as result ON {cj.result}={result.pk}}` +
	` WHERE %s` +
	` ORDER BY {cj.creationtime} DESC, {cj.code}`

var cronJobsCmd = &cobra.Command{
	Use:   "cronjobs",
	Short: "Show CronJob execution statistics via FlexibleSearch",
	Example: `  hac-helper cronjobs --cronjob myBackofficeJob
  hac-helper cronjobs --job myJob --like
  hac-helper cronjobs --bean myBean -f csv`,
	RunE: runCronJobs,
}

func init() {
	rootCmd.AddCommand(cronJobsCmd)
	f := cronJobsCmd.Flags()
	f.StringVarP(&cronJobsEnvironment, "environment", "e", defaultEnvironment, "environment code")
	f.StringVar(&cronJobsAspect, "aspect", defaultAspect, "aspect to target")
	f.StringVar(&cronJobsHACPath, "hac-path", defaultHACPath, "path to HAC within the aspect URL")
	f.StringVarP(&cronJobsCronJob, "cronjob", "c", "", "filter by CronJob code")
	f.StringVarP(&cronJobsJob, "job", "j", "", "filter by Job code")
	f.StringVarP(&cronJobsBean, "bean", "b", "", "filter by ServiceLayerJob spring bean ID")
	f.BoolVar(&cronJobsLike, "like", false, "use LIKE instead of exact match")
	f.IntVar(&cronJobsMaxCount, "max-count", 200, "maximum number of results")
	f.StringVarP(&cronJobsFormat, "format", "f", "table", "output format: table or csv")
}

func buildCronJobsCondition(field, value string, like bool) string {
	if like {
		return fmt.Sprintf("%s LIKE '%%%s%%'", field, value)
	}
	return fmt.Sprintf("%s = '%s'", field, value)
}

func runCronJobs(_ *cobra.Command, _ []string) error {
	var field, value string
	switch {
	case cronJobsCronJob != "":
		field, value = "{cj.code}", cronJobsCronJob
	case cronJobsJob != "":
		field, value = "{j.code}", cronJobsJob
	case cronJobsBean != "":
		field, value = "{sj.springId}", cronJobsBean
	default:
		return fmt.Errorf("one of --cronjob, --job, or --bean is required")
	}

	query := fmt.Sprintf(cronJobsQueryTemplate, buildCronJobsCondition(field, value, cronJobsLike))

	project, subscription, err := resolveProjectSub(defaultSubscription)
	if err != nil {
		return err
	}

	client, err := newHACClient(project, subscription, cronJobsEnvironment, cronJobsAspect, cronJobsHACPath)
	if err != nil {
		return err
	}

	spinner, _ := pterm.DefaultSpinner.Start("Fetching CronJob statistics...")
	result, err := client.ExecuteFlexSearch(hac.QueryRequest{Query: query, MaxCount: cronJobsMaxCount})
	if err != nil {
		spinner.Fail("Query failed")
		return err
	}
	spinner.Success(fmt.Sprintf("%d row(s) returned", result.ResultCount))

	return printQueryResults(result, cronJobsFormat)
}
