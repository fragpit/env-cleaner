package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	apiEnvironmentsEndpoint = "/api/environments"
)

var envCmd = &cobra.Command{
	Use:     "environment",
	Aliases: []string{"env"},
	Short:   "Operations with environments",
	Long:    `Environment command group allows to manage environments`,
	Run: func(cmd *cobra.Command, args []string) { //nolint:revive
		log.Info("environment called")
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
}
