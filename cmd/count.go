package cmd

import (
	"fmt"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	countEnvironment string
	countAspect      string
	countHACPath     string
	countWhere       string
)

var countCmd = &cobra.Command{
	Use:   "count <TypeCode>",
	Short: "Count items of a given type via FlexibleSearch",
	Example: `  hac-helper count Product
  hac-helper count Order --where "{status} = 'CREATED'"`,
	Args: cobra.ExactArgs(1),
	RunE: runCount,
}

func init() {
	rootCmd.AddCommand(countCmd)
	f := countCmd.Flags()
	f.StringVarP(&countEnvironment, "environment", "e", defaultEnvironment, "environment code")
	f.StringVar(&countAspect, "aspect", defaultAspect, "aspect to target")
	f.StringVar(&countHACPath, "hac-path", defaultHACPath, "path to HAC within the aspect URL")
	f.StringVar(&countWhere, "where", "", "optional WHERE clause (without the WHERE keyword)")
}

func buildCountQuery(typeCode, where string) string {
	q := fmt.Sprintf("SELECT count({PK}) FROM {%s}", typeCode)
	if where != "" {
		q += " WHERE " + where
	}
	return q
}

func runCount(_ *cobra.Command, args []string) error {
	typeCode := args[0]
	if err := validateTypeCode(typeCode); err != nil {
		return err
	}
	query := buildCountQuery(typeCode, countWhere)

	project, subscription, err := resolveProjectSub(defaultSubscription)
	if err != nil {
		return err
	}

	client, err := newHACClient(project, subscription, countEnvironment, countAspect, countHACPath)
	if err != nil {
		return err
	}

	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Counting %s...", typeCode))
	result, err := client.ExecuteFlexSearch(hac.QueryRequest{Query: query, MaxCount: 1})
	if err != nil {
		spinner.Fail("Query failed")
		return err
	}

	if len(result.Results) > 0 && len(result.Results[0]) > 0 {
		count := valueToString(result.Results[0][0])
		spinner.Success(fmt.Sprintf("%s: %s", typeCode, count))
	} else {
		spinner.Warning("No result returned")
	}

	return nil
}
