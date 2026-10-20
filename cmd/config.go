package cmd

import "github.com/spf13/cobra"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage hac configuration",
}

func init() {
	rootCmd.AddCommand(configCmd)
}
