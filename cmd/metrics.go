package cmd

import (
    "encoding/json"
    "fmt"

    "github.com/persys-dev/persysctl/internal/client"
    "github.com/persys-dev/persysctl/internal/config"
    "github.com/spf13/cobra"
)

var metricsCmd = &cobra.Command{
    Use:   "metrics",
    Short: "View Prow metrics",
    Long:  `Retrieves node and workload metrics from Prow.`,
    Run: func(cmd *cobra.Command, args []string) {
        c, err := client.NewClient(config.GetConfig())
        cobra.CheckErr(err)
        metrics, err := c.GetMetrics()
        cobra.CheckErr(err)
        data, err := json.MarshalIndent(metrics, "", "  ")
        cobra.CheckErr(err)
        fmt.Println(string(data))
    },
}

func init() {
    rootCmd.AddCommand(metricsCmd)
}