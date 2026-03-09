package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/fragpit/env-cleaner/internal/config"
)

var cfgFile string
var cfg *config.ClientConfig
var err error
var Debug bool
var version = "undefined"

var logLevel slog.LevelVar

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "env-cleaner",
	Short: "Automated environment cleaner",
	Long: `
Automated cleaner is a tool to clean up your environments within different
infrastructures.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) { //nolint:revive
		if Debug {
			logLevel.Set(slog.LevelDebug)
			slog.Info("debug mode enabled")
			for key, value := range viper.GetViper().AllSettings() {
				slog.Info("command flag", slog.String(key, fmt.Sprint(value)))
			}
		}

		cfg, err = config.NewClientConfig()
		if err != nil {
			slog.Error("error reading configuration", slog.Any("error", err))
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"",
		"config file (default is $HOME/.env-cleaner/env-cleaner.yaml)",
	)
	rootCmd.PersistentFlags().
		BoolVarP(&Debug, "debug", "d", false, "Enable debug mode (default: false)")

	if err := viper.BindPFlag(
		"debug",
		rootCmd.PersistentFlags().Lookup("debug"),
	); err != nil {
		slog.Error("error binding flag", slog.Any("error", err))
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     &logLevel,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					src.File = filepath.Base(src.File)
					a.Value = slog.AnyValue(src)
				}
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	cmd, _, err := rootCmd.Find(os.Args[1:])
	if err != nil {
		slog.Error("error finding command", slog.Any("error", err))
		return
	}

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		if cmd.Name() == "server" {
			viper.AddConfigPath(home + "/.env-cleaner/")
			viper.SetConfigType("yaml")
			viper.SetConfigName("env-cleaner.yml")
		} else {
			viper.AddConfigPath(home + "/.env-cleaner/")
			viper.SetConfigType("yaml")
			viper.SetConfigName("env-cleaner-client.yml")
		}
	}

	viper.SetEnvPrefix("EC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		slog.Info("using config file", slog.String("file", viper.ConfigFileUsed()))
	} else {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
