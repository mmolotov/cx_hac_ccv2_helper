package cmd

import (
	"fmt"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	enumEnvironment        string
	enumAspect             string
	enumHACPath            string
	enumLang               string
	enumFormat             string
	enumQueryTimeout       int
	enumCancelQueryTimeout int
)

var enumCodesCmd = &cobra.Command{
	Use:   "enumcodes <TypeCode>",
	Short: "List all codes for a SAP Commerce enum type",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnumCodes,
}

func init() {
	rootCmd.AddCommand(enumCodesCmd)
	f := enumCodesCmd.Flags()
	f.StringVarP(&enumEnvironment, "environment", "e", defaultEnvironment, "environment code")
	f.StringVar(&enumAspect, "aspect", defaultAspect, "aspect: api, backoffice, processing, storefront (or full CCV2 code)")
	f.StringVar(&enumHACPath, "hac-path", defaultHACPath, "path to HAC within the aspect URL")
	f.StringVarP(&enumLang, "lang", "l", "en", "language for localized name (e.g. en, de)")
	f.StringVarP(&enumFormat, "format", "f", "table", "output format: table or csv")
	f.IntVar(&enumQueryTimeout, "query-timeout", 0, "query timeout in milliseconds (0 = disabled)")
	f.IntVar(&enumCancelQueryTimeout, "cancel-query-timeout", 0, "cancel query timeout in milliseconds (0 = disabled)")
}

func runEnumCodes(_ *cobra.Command, args []string) error {
	if flagProject == "" {
		return fmt.Errorf("--project flag is required")
	}
	project := flagProject
	subscription := flagSubscription
	if subscription == "" {
		subscription = defaultSubscription
	}

	typeCode := args[0]
	if err := validateTypeCode(typeCode); err != nil {
		return err
	}
	if err := validateLocale(enumLang); err != nil {
		return err
	}
	query := fmt.Sprintf(
		"SELECT {CODE}, {NAME[%s]:o}, {PK} FROM {%s} ORDER BY {CODE}",
		enumLang, typeCode,
	)

	client, err := newHACClient(project, subscription, enumEnvironment, enumAspect, enumHACPath)
	if err != nil {
		return err
	}

	req := hac.QueryRequest{
		Query:              query,
		MaxCount:           1000,
		QueryTimeout:       enumQueryTimeout,
		CancelQueryTimeout: enumCancelQueryTimeout,
	}

	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Fetching enum codes for %s...", typeCode))
	result, err := client.ExecuteFlexSearch(req)
	if err != nil {
		spinner.Fail("Query failed")
		return err
	}
	spinner.Success(fmt.Sprintf("%d code(s) found", result.ResultCount))

	if flagDebug {
		pterm.Debug.Printf("Server execution time: %dms\n", result.ExecutionTime)
	}

	return printQueryResults(result, enumFormat)
}
