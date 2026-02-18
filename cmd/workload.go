package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/persys-dev/persysctl/internal/client"
	"github.com/persys-dev/persysctl/internal/config"
	controlv1 "github.com/persys-dev/persysctl/internal/controlv1"
	"github.com/persys-dev/persysctl/internal/models"
	agentv1 "github.com/persys/compute-agent/pkg/api/v1"
	"github.com/spf13/cobra"
)

var (
	workloadListStatus string
	workloadListNodeID string
	workloadGetID      string
	workloadDeleteID   string
	workloadRetryID    string
	workloadSpecFile   string
	workloadRevision   string
	workloadDesired    string
)

var workloadCmd = &cobra.Command{
	Use:   "workload",
	Short: "Manage workloads",
}

var workloadScheduleCmd = &cobra.Command{
	Use:   "schedule [flags] [file]",
	Short: "Schedule/apply a workload",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		var workload models.Workload
		if len(args) > 0 {
			data, err := os.ReadFile(args[0])
			cobra.CheckErr(err)
			cobra.CheckErr(json.Unmarshal(data, &workload))
		} else {
			workload.ID, _ = cmd.Flags().GetString("id")
			workload.Name, _ = cmd.Flags().GetString("name")
			workload.Type, _ = cmd.Flags().GetString("type")
			workload.Image, _ = cmd.Flags().GetString("image")
			workload.Command, _ = cmd.Flags().GetString("command")
			workload.Compose, _ = cmd.Flags().GetString("compose")
			workload.GitRepo, _ = cmd.Flags().GetString("git-repo")
			workload.GitBranch, _ = cmd.Flags().GetString("git-branch")
			workload.GitToken, _ = cmd.Flags().GetString("git-token")
			workload.LocalPath, _ = cmd.Flags().GetString("local-path")
			workload.Ports, _ = cmd.Flags().GetStringSlice("ports")
			workload.Volumes, _ = cmd.Flags().GetStringSlice("volumes")
			workload.Network, _ = cmd.Flags().GetString("network")
			workload.RestartPolicy, _ = cmd.Flags().GetString("restart-policy")
			envStr, _ := cmd.Flags().GetString("env")
			if envStr != "" {
				workload.EnvVars = parseEnvVars(envStr)
			}
		}

		if workload.Type == "" {
			cobra.CheckErr(fmt.Errorf("type is required"))
		}
		if !strings.Contains("docker-container,docker-compose,git-compose,container,compose", workload.Type) {
			cobra.CheckErr(fmt.Errorf("type must be docker-container, docker-compose, git-compose, container, or compose"))
		}
		if workload.Type == "docker-container" && workload.Image == "" {
			cobra.CheckErr(fmt.Errorf("image is required for docker-container"))
		}

		// Spec-file mode allows schedule syntax for direct gRPC scheduler/agent apply.
		if workloadSpecFile != "" {
			if cfg.Transport != "grpc" {
				cobra.CheckErr(fmt.Errorf("--spec-file is supported only with --transport grpc"))
			}
			if workload.ID == "" {
				cobra.CheckErr(fmt.Errorf("--id is required when using --spec-file"))
			}
			if workload.Type == "" {
				cobra.CheckErr(fmt.Errorf("--type is required when using --spec-file"))
			}
		}

		c, err := client.NewClient(cfg)
		cobra.CheckErr(err)
		defer c.Close()

		if workloadSpecFile != "" {
			switch cfg.GRPCTarget {
			case "scheduler":
				spec, err := buildSchedulerWorkloadSpec(workload.Type, workloadSpecFile)
				cobra.CheckErr(err)
				resp, err := c.ApplySchedulerWorkload(&controlv1.ApplyWorkloadRequest{
					WorkloadId:   workload.ID,
					RevisionId:   workloadRevision,
					DesiredState: normalizeDesiredState(workloadDesired),
					Spec:         spec,
				})
				cobra.CheckErr(err)
				printProto(resp)
				return
			case "agent":
				req, err := buildAgentApplyRequestFromSpec(workload.ID, workload.Type, workloadSpecFile, workloadRevision, workloadDesired)
				cobra.CheckErr(err)
				resp, err := c.ApplyAgentWorkload(req)
				cobra.CheckErr(err)
				printProto(resp)
				return
			default:
				cobra.CheckErr(fmt.Errorf("unsupported grpc target %q (expected scheduler or agent)", cfg.GRPCTarget))
			}
		}

		resp, err := c.ScheduleWorkload(workload)
		cobra.CheckErr(err)
		fmt.Printf("Workload %s scheduled on %s (status: %s)\n", resp.WorkloadID, resp.NodeID, resp.Status)
	},
}

var workloadListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workloads",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.NewClient(config.GetConfig())
		cobra.CheckErr(err)
		defer c.Close()

		workloads, err := c.ListWorkloads(workloadListNodeID, workloadListStatus)
		cobra.CheckErr(err)
		data, err := json.MarshalIndent(workloads, "", "  ")
		cobra.CheckErr(err)
		fmt.Println(string(data))
	},
}

var workloadGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get workload details (scheduler gRPC)",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.NewClient(config.GetConfig())
		cobra.CheckErr(err)
		defer c.Close()

		resp, err := c.GetWorkload(workloadGetID)
		cobra.CheckErr(err)
		printProto(resp)
	},
}

var workloadDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete workload (scheduler gRPC)",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.NewClient(config.GetConfig())
		cobra.CheckErr(err)
		defer c.Close()

		resp, err := c.DeleteWorkload(workloadDeleteID)
		cobra.CheckErr(err)
		printProto(resp)
	},
}

var workloadRetryCmd = &cobra.Command{
	Use:   "retry",
	Short: "Retry workload (scheduler gRPC)",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.NewClient(config.GetConfig())
		cobra.CheckErr(err)
		defer c.Close()

		resp, err := c.RetryWorkload(workloadRetryID)
		cobra.CheckErr(err)
		printProto(resp)
	},
}

func init() {
	rootCmd.AddCommand(workloadCmd)
	workloadCmd.AddCommand(workloadScheduleCmd)
	workloadCmd.AddCommand(workloadListCmd)
	workloadCmd.AddCommand(workloadGetCmd)
	workloadCmd.AddCommand(workloadDeleteCmd)
	workloadCmd.AddCommand(workloadRetryCmd)

	workloadScheduleCmd.Flags().String("id", "", "Workload ID")
	workloadScheduleCmd.Flags().String("name", "", "Workload name")
	workloadScheduleCmd.Flags().String("type", "", "Workload type")
	workloadScheduleCmd.Flags().String("image", "", "Docker image")
	workloadScheduleCmd.Flags().String("command", "", "Command")
	workloadScheduleCmd.Flags().String("compose", "", "Compose content (raw YAML or base64)")
	workloadScheduleCmd.Flags().String("git-repo", "", "Git repository URL")
	workloadScheduleCmd.Flags().String("git-branch", "main", "Git branch")
	workloadScheduleCmd.Flags().String("git-token", "", "Git auth token")
	workloadScheduleCmd.Flags().String("local-path", "", "Local Compose path")
	workloadScheduleCmd.Flags().String("env", "", "Environment variables (key1=value1,key2=value2)")
	workloadScheduleCmd.Flags().StringArray("ports", []string{}, "Ports to expose (e.g., 8080:80)")
	workloadScheduleCmd.Flags().StringArray("volumes", []string{}, "Volumes to mount (e.g., /host/path:/container/path)")
	workloadScheduleCmd.Flags().String("network", "", "Network for the container")
	workloadScheduleCmd.Flags().String("restart-policy", "no", "Restart policy")
	workloadScheduleCmd.Flags().StringVar(&workloadSpecFile, "spec-file", "", "Path to JSON spec file (gRPC mode)")
	workloadScheduleCmd.Flags().StringVar(&workloadRevision, "revision", "rev-1", "Workload revision ID (spec-file mode)")
	workloadScheduleCmd.Flags().StringVar(&workloadDesired, "desired-state", "running", "Desired state: running|stopped (spec-file mode)")

	workloadListCmd.Flags().StringVar(&workloadListStatus, "status", "", "Filter by status (scheduler target)")
	workloadListCmd.Flags().StringVar(&workloadListNodeID, "node-id", "", "Filter by node id (scheduler target)")

	workloadGetCmd.Flags().StringVar(&workloadGetID, "id", "", "Workload ID")
	workloadDeleteCmd.Flags().StringVar(&workloadDeleteID, "id", "", "Workload ID")
	workloadRetryCmd.Flags().StringVar(&workloadRetryID, "id", "", "Workload ID")
	cobra.CheckErr(workloadGetCmd.MarkFlagRequired("id"))
	cobra.CheckErr(workloadDeleteCmd.MarkFlagRequired("id"))
	cobra.CheckErr(workloadRetryCmd.MarkFlagRequired("id"))
}

func buildAgentApplyRequestFromSpec(id, typ, specFile, revision, desired string) (*agentv1.ApplyWorkloadRequest, error) {
	specData, err := os.ReadFile(specFile)
	if err != nil {
		return nil, err
	}

	req := &agentv1.ApplyWorkloadRequest{
		Id:           id,
		RevisionId:   revision,
		DesiredState: desiredState(desired),
		Spec:         &agentv1.WorkloadSpec{},
	}

	switch strings.ToLower(typ) {
	case "container", "docker-container":
		req.Type = agentv1.WorkloadType_WORKLOAD_TYPE_CONTAINER
		container := &agentv1.ContainerSpec{}
		if err := json.Unmarshal(specData, container); err != nil {
			return nil, err
		}
		req.Spec.Spec = &agentv1.WorkloadSpec_Container{Container: container}
	case "compose", "docker-compose":
		req.Type = agentv1.WorkloadType_WORKLOAD_TYPE_COMPOSE
		compose := &agentv1.ComposeSpec{}
		if err := json.Unmarshal(specData, compose); err != nil {
			return nil, err
		}
		req.Spec.Spec = &agentv1.WorkloadSpec_Compose{Compose: compose}
	case "vm":
		req.Type = agentv1.WorkloadType_WORKLOAD_TYPE_VM
		vm := &agentv1.VMSpec{}
		if err := json.Unmarshal(specData, vm); err != nil {
			return nil, err
		}
		req.Spec.Spec = &agentv1.WorkloadSpec_Vm{Vm: vm}
	default:
		return nil, fmt.Errorf("unsupported --type %q, use container|compose|vm", typ)
	}

	return req, nil
}

func parseEnvVars(envStr string) map[string]string {
	envVars := make(map[string]string)
	pairs := strings.Split(envStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			envVars[kv[0]] = kv[1]
		}
	}
	return envVars
}
