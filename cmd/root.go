package cmd

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/fragpit/env-cleaner/internal/config"
	"github.com/fragpit/env-cleaner/internal/model"
)

var cfgFile string
var cfg *config.ClientConfig
var err error
var Debug bool
var version = "undefined"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "env-cleaner",
	Short: "Automated environment cleaner",
	Long: `
Automated cleaner is a tool to clean up your environments within different
infrastructures.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) { //nolint:revive
		if Debug {
			log.Info("Debug mode enabled")
			for key, value := range viper.GetViper().AllSettings() {
				log.WithFields(log.Fields{
					key: value,
				}).Info("Command Flag")
			}
		}

		cfg, err = config.NewClientConfig()
		if err != nil {
			log.Fatalf("Error reading configuration: %v", err)
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

	if err := viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		log.Error(err)
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	cmd, _, err := rootCmd.Find(os.Args[1:])
	if err != nil {
		log.Error(err)
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
		log.Info("Using config file:", viper.ConfigFileUsed())
	} else {
		log.Fatal(err)
	}
}

func setName(env *model.Environment) string {
	name := env.Name
	if env.Namespace != "" {
		name = fmt.Sprintf("%s (namespace: %s)", env.Name, env.Namespace)
	}

	return name
}
