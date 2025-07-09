package cmd

import (
    "fmt"
    "os"

    "github.com/persys-dev/persys-cli/internal/config"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var cfgFile string
var verbose bool

var rootCmd = &cobra.Command{
    Use:   "persys-cli",
    Short: "Persys CLI for interacting with Persys",
    Long:  `A command-line client for managing workloads, nodes, and metrics in the Persys system.`,
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(initConfig)
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.persys/config.yaml)")
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}

func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, err := os.UserHomeDir()
        cobra.CheckErr(err)
        viper.AddConfigPath(home + "/.persys")
        viper.SetConfigName("config")
        viper.SetConfigType("yaml")
    }

    viper.AutomaticEnv()
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            cobra.CheckErr(fmt.Errorf("failed to read config: %v", err))
        }
    }

    config.InitLogger(verbose)
}