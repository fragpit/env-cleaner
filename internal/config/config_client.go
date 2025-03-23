package config

import (
	"github.com/spf13/viper"
)

type ClientConfig struct {
	APIURL      string `mapstructure:"api_url"`
	AdminAPIKey string `mapstructure:"admin_api_key"`
}

func NewClientConfig() (*ClientConfig, error) {
	var cfg ClientConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
