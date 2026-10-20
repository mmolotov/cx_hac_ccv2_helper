package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	flexEnvironment        string
	flexAspect             string
	flexHACPath            string
	flexSQL                bool
	flexMaxCount           int
	flexFormat             string
	flexLocale             string
	flexQueryTimeout       int
	flexCancelQueryTimeout int
)

var flexSearchCmd = &cobra.Command{
	Use:   "flexsearch [query]",
	Short: "Execute a FlexibleSearch or SQL query via HAC",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runFlexSearch,
}

func init() {
	rootCmd.AddCommand(flexSearchCmd)
	f := flexSearchCmd.Flags()
	f.StringVarP(&flexEnvironment, "environment", "e", defaultEnvironment, "environment code")
	f.StringVar(&flexAspect, "aspect", defaultAspect, "aspect: api, backoffice, processing, storefront (or full CCV2 code)")
	f.StringVar(&flexHACPath, "hac-path", defaultHACPath, "path to HAC within the aspect URL")
	f.BoolVar(&flexSQL, "sql", false, "execute as direct SQL (default: FlexibleSearch)")
	f.IntVarP(&flexMaxCount, "max-count", "n", 200, "maximum number of results")
	f.StringVarP(&flexFormat, "format", "f", "table", "output format: table or csv")
	f.StringVarP(&flexLocale, "locale", "l", "", "locale for query execution (e.g. en, de)")
	f.IntVar(&flexQueryTimeout, "query-timeout", 0, "query timeout in milliseconds (0 = disabled)")
	f.IntVar(&flexCancelQueryTimeout, "cancel-query-timeout", 0, "cancel query timeout in milliseconds (0 = disabled)")
}

func runFlexSearch(_ *cobra.Command, args []string) error {
	if flagProject == "" {
		return fmt.Errorf("--project flag is required")
	}
	project := flagProject
	subscription := flagSubscription
	if subscription == "" {
		subscription = defaultSubscription
	}

	// Resolve query: positional arg or stdin.
	var query string
	if len(args) > 0 {
		query = args[0]
	} else {
		stat, _ := os.Stdin.Stat()
		if stat.Mode()&os.ModeCharDevice == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading query from stdin: %w", err)
			}
			query = strings.TrimSpace(string(data))
		}
	}
	if query == "" {
		return fmt.Errorf("provide a query as an argument or via stdin")
	}

	client, err := newHACClient(project, subscription, flexEnvironment, flexAspect, flexHACPath)
	if err != nil {
		return err
	}

	req := hac.QueryRequest{
		Query:              query,
		MaxCount:           flexMaxCount,
		Locale:             flexLocale,
		QueryTimeout:       flexQueryTimeout,
		CancelQueryTimeout: flexCancelQueryTimeout,
	}

	spinner, _ := pterm.DefaultSpinner.Start("Executing query...")
	var result *hac.QueryResult
	if flexSQL {
		result, err = client.ExecuteSQL(req)
	} else {
		result, err = client.ExecuteFlexSearch(req)
	}
	if err != nil {
		spinner.Fail("Query failed")
		return err
	}
	spinner.Success(fmt.Sprintf("%d row(s) returned", result.ResultCount))

	if flagDebug {
		pterm.Debug.Printf("Server execution time: %dms\n", result.ExecutionTime)
	}

	return printQueryResults(result, flexFormat)
}

// valueToString converts a JSON-decoded value to a human-readable string.
// JSON numbers come in as float64; integers are formatted without decimals.
func valueToString(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	if f, ok := v.(float64); ok {
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

func printQueryResults(result *hac.QueryResult, format string) error {
	switch format {
	case "csv":
		return printQueryCSV(os.Stdout, result)
	default:
		return printQueryTable(os.Stdout, result)
	}
}

func printQueryTable(out io.Writer, result *hac.QueryResult) error {
	if len(result.Headers) == 0 && len(result.Results) == 0 {
		pterm.Info.Println("No results.")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(result.Headers, "\t"))
	for _, row := range result.Results {
		strs := make([]string, len(row))
		for i, v := range row {
			strs[i] = valueToString(v)
		}
		fmt.Fprintln(w, strings.Join(strs, "\t"))
	}
	return w.Flush()
}

func printQueryCSV(out io.Writer, result *hac.QueryResult) error {
	w := csv.NewWriter(out)
	w.Comma = ';'
	if len(result.Headers) > 0 {
		if err := w.Write(result.Headers); err != nil {
			return err
		}
	}
	for _, row := range result.Results {
		strs := make([]string, len(row))
		for i, v := range row {
			strs[i] = valueToString(v)
		}
		if err := w.Write(strs); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
