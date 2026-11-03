package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sdmoradi/pvbench/internal/benchmark"
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

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pvbench version %s\n", version)
		},
	})

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

			info, err := inspect.PVC(context.Background(), client, namespace, args[0])
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
	inspectCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace (defaults to the current context namespace)")
	cmd.AddCommand(inspectCmd)

	var storageClass, size, profile, timeoutText string
	var keepResources bool
	benchmarkCmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Benchmark a StorageClass using a temporary PVC and FIO",
		Long: `Creates a disposable PVC and Job, runs FIO against a test file,
prints the FIO output, and removes resources created by the run.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, contextNamespace, err := kube.NewClient()
			if err != nil {
				return fmt.Errorf("initialize Kubernetes client: %w", err)
			}
			namespace, err := cmd.Flags().GetString("namespace")
			if err != nil {
				return err
			}
			if namespace == "" {
				namespace = contextNamespace
			}
			timeout, err := time.ParseDuration(timeoutText)
			if err != nil || timeout <= 0 {
				return fmt.Errorf("invalid --timeout %q (example: 5m)", timeoutText)
			}
			return benchmark.Run(context.Background(), client, benchmark.Options{
				Namespace: namespace,
				StorageClass: storageClass,
				Size: size,
				Profile: profile,
				Timeout: timeout,
				KeepResources: keepResources,
			}, cmd.OutOrStdout())
		},
	}
	benchmarkCmd.Flags().StringVar(&storageClass, "storage-class", "", "StorageClass to benchmark (required)")
	_ = benchmarkCmd.MarkFlagRequired("storage-class")
	benchmarkCmd.Flags().StringVar(&size, "size", "1Gi", "Size of the temporary benchmark PVC (minimum 1Gi)")
	benchmarkCmd.Flags().StringVar(&profile, "profile", "all", "Workload profile: read, write, or all")
	benchmarkCmd.Flags().StringVar(&timeoutText, "timeout", "5m", "Maximum duration to wait for the benchmark (e.g. 5m)")
	benchmarkCmd.Flags().BoolVar(&keepResources, "keep-resources", false, "Keep the temporary Job and PVC for debugging")
	benchmarkCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace (defaults to the current context namespace)")
	cmd.AddCommand(benchmarkCmd)

	return cmd
}
