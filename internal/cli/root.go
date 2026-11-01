package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl-pvbench",
		Short: "Kubernetes persistent volume inspection and benchmarking",
		Long: `pvbench helps inspect and benchmark Kubernetes
persistent volumes using controlled storage workloads.`,
	}

	cmd.Version = version

	cmd.SetVersionTemplate("pvbench version {{.Version}}\n")

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pvbench version %s\n", version)
		},
	})

	return cmd
}
