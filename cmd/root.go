package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "opwriting",
	Short: "Overpowered writing tools",
	Long:  "A CLI tool for managing writing repositories with post directories across Git branches",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(publishCmd)
}