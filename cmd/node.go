package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/persys-dev/persysctl/internal/client"
	"github.com/persys-dev/persysctl/internal/config"
	"github.com/spf13/cobra"
)

var (
	nodeListStatus string
	nodeGetID      string
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage nodes",
}

var nodeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List nodes",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.NewClient(config.GetConfig())
		cobra.CheckErr(err)
		defer c.Close()

		nodes, err := c.ListNodes(nodeListStatus)
		cobra.CheckErr(err)
		data, err := json.MarshalIndent(nodes, "", "  ")
		cobra.CheckErr(err)
		fmt.Println(string(data))
	},
}

var nodeGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get node details (scheduler gRPC)",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.NewClient(config.GetConfig())
		cobra.CheckErr(err)
		defer c.Close()

		resp, err := c.GetNode(nodeGetID)
		cobra.CheckErr(err)
		printProto(resp)
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeListCmd)
	nodeCmd.AddCommand(nodeGetCmd)

	nodeListCmd.Flags().StringVar(&nodeListStatus, "status", "", "Filter by status: Ready|NotReady|Draining")
	nodeGetCmd.Flags().StringVar(&nodeGetID, "id", "", "Node ID")
	cobra.CheckErr(nodeGetCmd.MarkFlagRequired("id"))
}
