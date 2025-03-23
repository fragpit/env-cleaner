package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/fragpit/env-cleaner/internal/server"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server mode",
	Long: `Server mode is a common mode for this application. It starts an API
interface, and schedules a crawler and cleanup job for the specified environments.`,
	Run: func(cmd *cobra.Command, args []string) { //nolint:revive
		log.Infof("Version: %s", version)
		if err := server.Run(); err != nil {
			log.Fatalf("Fatal error: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
