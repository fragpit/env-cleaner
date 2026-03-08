package cmd

import (
	"log/slog"
	"os"

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
		slog.Info("starting server", slog.String("version", version))
		if err := server.Run(); err != nil {
			slog.Error("fatal error", slog.Any("error", err))
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
