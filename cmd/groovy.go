package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/hac"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	groovyEnvironment string
	groovyAspects     []string
	groovyAllAspects  bool
	groovyCommit      bool
	groovyNode        string
	groovyHACPath     string
)

var groovyCmd = &cobra.Command{
	Use:   "groovy [script | file.groovy | ./scripts/]",
	Short: "Execute Groovy scripts via HAC",
	Long: `Execute Groovy scripts via HAC.

The argument can be:
  - a path to a .groovy file
  - a path to a directory (all .groovy files executed in alphabetical order)
  - an inline script string
  - omitted: read from stdin`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGroovy,
}

func init() {
	rootCmd.AddCommand(groovyCmd)
	f := groovyCmd.Flags()
	f.StringVarP(&groovyEnvironment, "environment", "e", defaultEnvironment, "environment code")
	f.StringSliceVarP(&groovyAspects, "aspect", "a", []string{defaultAspect}, "aspect(s): api, backoffice, processing, storefront (or full CCV2 code)")
	f.BoolVar(&groovyAllAspects, "all-aspects", false, "execute on all configured aspects")
	f.BoolVar(&groovyCommit, "commit", false, "commit mode (default: false = rollback)")
	f.StringVar(&groovyNode, "node", "", "target a specific cluster node by replica name (sets ROUTE cookie)")
	f.StringVar(&groovyHACPath, "hac-path", defaultHACPath, "path to HAC within the aspect URL")
}

func runGroovy(_ *cobra.Command, args []string) error {
	if flagProject == "" {
		return fmt.Errorf("--project flag is required")
	}
	scripts, err := resolveScripts(args)
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
		Environment:  groovyEnvironment,
		Aspects:      groovyAspects,
		AllAspects:   groovyAllAspects,
		HACPath:      groovyHACPath,
		Node:         groovyNode,
	}, func(client *hac.Client, _ hac.Target) error {
		return executeScripts(client, scripts)
	})
}

// namedScript is a script with a display name (filename or "inline").
type namedScript struct {
	name   string
	script string
}

// executeScripts runs all scripts sequentially on a single authenticated client.
func executeScripts(client *hac.Client, scripts []namedScript) error {
	multiScript := len(scripts) > 1
	var failed []string

	for _, s := range scripts {
		if multiScript {
			pterm.DefaultSection.Printf("Script: %s\n", s.name)
		}
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Executing %s...", s.name))
		result, err := client.ExecuteScript(hac.ScriptRequest{
			Script: s.script,
			Commit: groovyCommit,
		})
		if err != nil {
			spinner.Fail(fmt.Sprintf("Request failed: %v", err))
			if multiScript {
				failed = append(failed, s.name)
				continue
			}
			return err
		}
		if result.HasError() {
			spinner.Fail("Script error")
		} else {
			spinner.Success("Done")
		}
		if err := printScriptResult(result); err != nil {
			if multiScript {
				failed = append(failed, s.name)
			} else {
				return err
			}
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed scripts: %s", strings.Join(failed, ", "))
	}
	return nil
}

func printScriptResult(result *hac.ScriptResult) error {
	if result.OutputText != "" {
		fmt.Println(strings.TrimRight(result.OutputText, "\n"))
	}
	if result.ExecutionResult != "" && result.ExecutionResult != "null" {
		pterm.Info.Printf("Result: %s\n", result.ExecutionResult)
	}
	if result.HasError() {
		msg := strings.SplitN(result.StacktraceText, "\n", 2)[0]
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// resolveScripts determines the script(s) to execute from the CLI arguments.
func resolveScripts(args []string) ([]namedScript, error) {
	if len(args) == 0 {
		stat, _ := os.Stdin.Stat()
		if stat.Mode()&os.ModeCharDevice == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("reading script from stdin: %w", err)
			}
			return []namedScript{{name: "stdin", script: strings.TrimSpace(string(data))}}, nil
		}
		return nil, fmt.Errorf("provide a script string, file path, directory, or pipe via stdin")
	}

	arg := args[0]
	info, err := os.Stat(arg)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %q: %w", arg, err)
	}
	if err == nil && info.IsDir() {
		return loadScriptsFromDir(arg)
	}
	if err == nil && info.Mode().IsRegular() {
		data, err := os.ReadFile(arg)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", arg, err)
		}
		return []namedScript{{name: filepath.Base(arg), script: string(data)}}, nil
	}
	// Treat as inline script string.
	return []namedScript{{name: "inline", script: arg}}, nil
}

// loadScriptsFromDir loads all *.groovy files from a directory in alphabetical order.
func loadScriptsFromDir(dir string) ([]namedScript, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %q: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".groovy") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .groovy files found in %q", dir)
	}
	sort.Strings(files)

	scripts := make([]namedScript, 0, len(files))
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", name, err)
		}
		scripts = append(scripts, namedScript{name: name, script: string(data)})
	}
	pterm.Info.Printf("Found %d script(s) in %q\n", len(scripts), dir)
	return scripts, nil
}
