package cmd

import (
    "encoding/json"
    "fmt"

    "github.com/persys-dev/persysctl/internal/client"
    "github.com/persys-dev/persysctl/internal/config"
    "github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
    Use:   "node",
    Short: "Manage nodes in Prow",
    Long:  `Commands to list nodes.`,
}

var nodeListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all nodes",
    Long:  `Retrieves a list of all nodes in Prow.`,
    Run: func(cmd *cobra.Command, args []string) {
        c, err := client.NewClient(config.GetConfig())
        cobra.CheckErr(err)
        nodes, err := c.ListNodes()
        cobra.CheckErr(err)
        data, err := json.MarshalIndent(nodes, "", "  ")
        cobra.CheckErr(err)
        fmt.Println(string(data))
    },
}

func init() {
    rootCmd.AddCommand(nodeCmd)
    nodeCmd.AddCommand(nodeListCmd)
}
