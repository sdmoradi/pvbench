package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sdmoradi/pvbench/internal/inspect"
	"github.com/sdmoradi/pvbench/internal/kube"
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

	// Version command
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pvbench version %s\n", version)
		},
	})

	// Inspect PVC command
	inspectCmd := &cobra.Command{
		Use:   "inspect PVC_NAME",
		Short: "Inspect a Kubernetes persistent volume claim",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, err := cmd.Flags().GetString("namespace")
			if err != nil {
				return err
			}

			client, contextNamespace, err := kube.NewClient()
			if err != nil {
				return fmt.Errorf("initialize Kubernetes client: %w", err)
			}

			if namespace == "" {
				namespace = contextNamespace
			}

			info, err := inspect.PVC(
				context.Background(),
				client,
				namespace,
				args[0],
			)
			if err != nil {
				return err
			}

			fmt.Printf("PVC INFORMATION\n\n")
			fmt.Printf("Name:          %s\n", info.Name)
			fmt.Printf("Namespace:     %s\n", info.Namespace)
			fmt.Printf("Status:        %s\n", info.Status)
			fmt.Printf("Capacity:      %s\n", info.Capacity)
			fmt.Printf("Access Modes:  %v\n", info.AccessModes)
			fmt.Printf("Volume Mode:   %s\n", info.VolumeMode)

			fmt.Printf("\nSTORAGE INFORMATION\n\n")
			fmt.Printf("PV:            %s\n", info.PVName)
			fmt.Printf("StorageClass:  %s\n", info.StorageClass)
			fmt.Printf("Provisioner:   %s\n", info.Provisioner)
			fmt.Printf("ReclaimPolicy: %s\n", info.ReclaimPolicy)

			return nil
		},
	}

	// Namespace flag belongs to the inspect command.
	inspectCmd.Flags().StringP(
		"namespace",
		"n",
		"",
		"Kubernetes namespace (defaults to the current context namespace)",
	)

	cmd.AddCommand(inspectCmd)

	return cmd
}
