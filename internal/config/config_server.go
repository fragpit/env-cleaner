package config

import (
	"github.com/spf13/viper"
)

type ServerConfig struct {
	APIURL            string        `mapstructure:"api_url"`
	AdminAPIKey       string        `mapstructure:"admin_api_key"`
	DryRun            bool          `mapstructure:"dry_run"`
	DefaultTTL        string        `mapstructure:"default_ttl"`
	SQLite            SQLite        `mapstructure:"sqlite"`
	Postgresql        Postgresql    `mapstructure:"postgresql"`
	MaxExtendDuration string        `mapstructure:"max_extend_duration"`
	OrphansReport     bool          `mapstructure:"orphans_report"`
	CrawlInterval     string        `mapstructure:"crawl_interval"`
	DeleteInterval    string        `mapstructure:"delete_interval"`
	StaleThreshold    string        `mapstructure:"stale_threshold"`
	Notifications     Notifications `mapstructure:"notifications"`
	Environments      Environments  `mapstructure:"environments"`
	Connectors        Connectors    `mapstructure:"connectors"`
}

type SQLite struct {
	DatabaseFolder string `mapstructure:"database_folder"`
}

type Postgresql struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type Notifications struct {
	AdminOnly bool  `mapstructure:"admin_only"`
	Slack     Slack `mapstructure:"slack"`
	Email     Email `mapstructure:"email"`
}

type Slack struct {
	Enabled      bool   `mapstructure:"enabled"`
	WebhookURL   string `mapstructure:"webhook_url"`
	SenderName   string `mapstructure:"sender_name"`
	AdminChannel string `mapstructure:"admin_channel"`
}

type Email struct {
	Enabled           bool   `mapstructure:"enabled"`
	SMTPServerAddress string `mapstructure:"smtp_server_address"`
	SMTPServerPort    int    `mapstructure:"smtp_server_port"`
	Username          string `mapstructure:"username"`
	Password          string `mapstructure:"password"`
	SenderEmail       string `mapstructure:"sender_email"`
	AdminEmail        string `mapstructure:"admin_email"`
}

type Environments struct {
	Helm      Helm      `mapstructure:"helm"`
	VSphereVM VSphereVM `mapstructure:"vsphere_vm"`
}

type Helm struct {
	Enabled                bool         `mapstructure:"enabled"`
	DeleteReleaseNamespace bool         `mapstructure:"delete_release_namespace"`
	VeleroBackup           VeleroBackup `mapstructure:"velero_backup"`
	WhitelistReleasesRegex []string     `mapstructure:"whitelist_releases_regex"`
	BlacklistNamespaces    []string     `mapstructure:"blacklist_namespaces"`
}

type VeleroBackup struct {
	Enabled   bool   `mapstructure:"enabled"`
	Namespace string `mapstructure:"namespace"`
	TTL       string `mapstructure:"ttl"`
}

type VSphereVM struct {
	Enabled            bool     `mapstructure:"enabled"`
	QuarantineFolderID string   `mapstructure:"quarantine_folder_id"`
	QuarantinePostfix  string   `mapstructure:"quarantine_postfix"`
	WatchFolders       []string `mapstructure:"watch_folders"`
	BlacklistVMs       []string `mapstructure:"blacklist_vms"`
}

type Connectors struct {
	K8s     K8s     `mapstructure:"k8s"`
	VSphere VSphere `mapstructure:"vsphere"`
}

type K8s struct {
	Insecure   bool   `mapstructure:"insecure"`
	Kubeconfig string `mapstructure:"kubeconfig"`
}

type VSphere struct {
	Insecure   bool   `mapstructure:"insecure"`
	Hostname   string `mapstructure:"hostname"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Datacenter string `mapstructure:"datacenter"`
}

func NewServerConfig() (*ServerConfig, error) {
	var cfg ServerConfig

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
