package cmd

import "github.com/spf13/cobra"

var configurationCmd = &cobra.Command{
	Use:     "configuration",
	Aliases: []string{"cfg"},
	Short:   "Read and write SAP platform configuration properties via HAC",
}

func init() {
	rootCmd.AddCommand(configurationCmd)
}
