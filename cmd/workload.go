package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/persys-dev/persys-cli/internal/client"
	"github.com/persys-dev/persys-cli/internal/config"
	"github.com/persys-dev/persys-cli/internal/models"
	"github.com/spf13/cobra"
)

var workloadCmd = &cobra.Command{
    Use:   "workload",
    Short: "Manage workloads in Prow",
    Long:  `Commands to schedule and list workloads.`,
}

var workloadScheduleCmd = &cobra.Command{
    Use:   "schedule [flags] [file]",
    Short: "Schedule a new workload",
    Long:  `Schedules a workload (Docker container, local Compose, or Git-based Compose). Provide a JSON file or use flags.`,
    Run: func(cmd *cobra.Command, args []string) {
        var workload models.Workload
        if len(args) > 0 {
            data, err := ioutil.ReadFile(args[0])
            cobra.CheckErr(err)
            err = json.Unmarshal(data, &workload)
            cobra.CheckErr(err)
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

        if workload.Name == "" || workload.Type == "" {
            cobra.CheckErr(fmt.Errorf("id, name, and type are required"))
        }
        if !strings.Contains("docker-container,docker-compose,git-compose", workload.Type) {
            cobra.CheckErr(fmt.Errorf("type must be docker-container, docker-compose, or git-compose"))
        }
        if workload.Type == "docker-container" && (workload.Image == "") {
            cobra.CheckErr(fmt.Errorf("image and command required for docker-container"))
        }
        if workload.Type == "docker-compose" && workload.LocalPath == "" {
            cobra.CheckErr(fmt.Errorf("local-path required for docker-compose"))
        }
        if workload.Type == "git-compose" && workload.GitRepo == "" {
            cobra.CheckErr(fmt.Errorf("git-repo required for git-compose"))
        }

        c, err := client.NewClient(config.GetConfig())
        cobra.CheckErr(err)
        resp, err := c.ScheduleWorkload(workload)
        cobra.CheckErr(err)
        fmt.Printf("Workload %s scheduled on node %s\n", resp.WorkloadID, resp.NodeID)
    },
}

var workloadListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all workloads",
    Long:  `Retrieves a list of all workloads in Prow.`,
    Run: func(cmd *cobra.Command, args []string) {
        c, err := client.NewClient(config.GetConfig())
        cobra.CheckErr(err)
        workloads, err := c.ListWorkloads()
        cobra.CheckErr(err)
        data, err := json.MarshalIndent(workloads, "", "  ")
        cobra.CheckErr(err)
        fmt.Println(string(data))
    },
}

func init() {
    rootCmd.AddCommand(workloadCmd)
    workloadCmd.AddCommand(workloadScheduleCmd)
    workloadCmd.AddCommand(workloadListCmd)

    workloadScheduleCmd.Flags().String("id", "", "Workload ID")
    workloadScheduleCmd.Flags().String("name", "", "Workload name")
    workloadScheduleCmd.Flags().String("type", "", "Workload type (docker-container, docker-compose, git-compose)")
    workloadScheduleCmd.Flags().String("image", "", "Docker image (for docker-container)")
    workloadScheduleCmd.Flags().String("command", "", "Command (for docker-container)")
    workloadScheduleCmd.Flags().String("compose", "", "Base64-encoded Compose content")
    workloadScheduleCmd.Flags().String("git-repo", "", "Git repository URL (for git-compose)")
    workloadScheduleCmd.Flags().String("git-branch", "main", "Git branch (for git-compose)")
    workloadScheduleCmd.Flags().String("git-token", "", "Git auth token (for git-compose)")
    workloadScheduleCmd.Flags().String("local-path", "", "Local Compose path (for docker-compose)")
    workloadScheduleCmd.Flags().String("env", "", "Environment variables (key1=value1,key2=value2)")
	workloadScheduleCmd.Flags().StringArray("ports", []string{}, "Ports to expose (e.g., 8080:80)")
	workloadScheduleCmd.Flags().StringArray("volumes", []string{}, "Volumes to mount (e.g., /host/path:/container/path)")
	workloadScheduleCmd.Flags().String("network", "", "Network for the container")
	workloadScheduleCmd.Flags().String("restart-policy", "no", "Restart policy (e.g., always, no-restart)")
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