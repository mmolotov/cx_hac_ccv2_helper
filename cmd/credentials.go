package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var credsCmd = &cobra.Command{
	Use:   "credentials",
	Short: "Manage the encrypted credentials vault",
}

var credsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects stored in the credentials vault",
	RunE:  runCredsList,
}

var credsDeleteCmd = &cobra.Command{
	Use:   "delete <project>",
	Short: "Delete credentials for a project from the vault",
	Args:  cobra.ExactArgs(1),
	RunE:  runCredsDelete,
}

func init() {
	rootCmd.AddCommand(credsCmd)
	credsCmd.AddCommand(credsListCmd, credsDeleteCmd)
}

func runCredsList(_ *cobra.Command, _ []string) error {
	masterPwd, err := ui.PromptPassword("Vault master password")
	if err != nil {
		return err
	}
	store, err := config.LoadCredentials(masterPwd)
	if err != nil {
		return fmt.Errorf("loading credentials vault: %w", err)
	}

	if len(store.Projects) == 0 {
		pterm.Info.Println("No credentials stored in vault.")
		return nil
	}

	projects := make([]string, 0, len(store.Projects))
	for name := range store.Projects {
		projects = append(projects, name)
	}
	sort.Strings(projects)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tTYPE\tDETAILS")
	for _, name := range projects {
		pc := store.Projects[name]
		if pc.HACPassword != "" {
			fmt.Fprintf(w, "%s\tdirect\tHAC password: set\n", name)
		} else if len(pc.Subscriptions) > 0 {
			subs := make([]string, 0, len(pc.Subscriptions))
			for alias, sc := range pc.Subscriptions {
				entry := alias
				if sc.SAPLogin != "" {
					entry += " (" + sc.SAPLogin + ")"
				}
				subs = append(subs, entry)
			}
			sort.Strings(subs)
			fmt.Fprintf(w, "%s\tccv2\t%s\n", name, strings.Join(subs, ", "))
		} else {
			fmt.Fprintf(w, "%s\t-\t(empty)\n", name)
		}
	}
	return w.Flush()
}

func runCredsDelete(_ *cobra.Command, args []string) error {
	project := args[0]

	masterPwd, err := ui.PromptPassword("Vault master password")
	if err != nil {
		return err
	}
	store, err := config.LoadCredentials(masterPwd)
	if err != nil {
		return fmt.Errorf("loading credentials vault: %w", err)
	}

	if _, ok := store.Projects[project]; !ok {
		return fmt.Errorf("project %q not found in credentials vault", project)
	}

	ok, err := ui.PromptConfirm(fmt.Sprintf("Delete credentials for project %q?", project))
	if err != nil {
		return err
	}
	if !ok {
		pterm.Info.Println("Aborted.")
		return nil
	}

	delete(store.Projects, project)
	if err := config.SaveCredentials(store, masterPwd); err != nil {
		return fmt.Errorf("saving credentials vault: %w", err)
	}

	pterm.Success.Printf("Credentials for %q removed from vault.\n", project)
	return nil
}
