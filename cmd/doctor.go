package cmd

import "github.com/spf13/cobra"

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate config, find orphan articles or broken sidecars",
	RunE:  stub("doctor"),
}

func init() { rootCmd.AddCommand(doctorCmd) }
