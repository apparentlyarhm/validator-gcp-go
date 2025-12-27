package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	SigningSecret string `envconfig:"SIGNING_SECRET" required:"true"`
	GitHub        GitHubConfig
	GoogleCloud   GoogleCloudConfig
	Minecraft     MinecraftConfig
}

type GitHubConfig struct {
	ClientId     string `envconfig:"GITHUB_CLIENT_ID" required:"true"`
	ClientSecret string `envconfig:"GITHUB_CLIENT_SECRET" required:"true"`
}

type GoogleCloudConfig struct {
	Project                string `envconfig:"GOOGLE_CLOUD_PROJECT" required:"true"`
	BucketName             string `envconfig:"GOOGLE_CLOUD_BUCKET_NAME" required:"true"`
	FirewallName           string `envconfig:"GOOGLE_CLOUD_FIREWALL_NAME" required:"true"`
	VMName                 string `envconfig:"GOOGLE_CLOUD_VM_NAME" required:"true"`
	VMZone                 string `envconfig:"GOOGLE_CLOUD_VM_ZONE" required:"true"`
	ApplicationCredentials string `envconfig:"GOOGLE_APPLICATION_CREDENTIALS" required:"true"`
}

type MinecraftConfig struct {
	RconPass   string `envconfig:"MINECRAFT_RCON_PASS" required:"true"`
	RconPort   int    `envconfig:"MINECRAFT_RCON_PORT" required:"true"`
	ServerPort int    `envconfig:"MINECRAFT_SERVER_PORT" required:"true"`
}

// github user IDs of admins - mostly for admin access related apis
var Admins = []string{
	"169424843",
}

// github user IDs of allowed users - mostly for normal rcon commands
var Users = []string{
	"103031918",
}

func Load() (Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return cfg, fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("[ENV] Loaded %v admins and %v users for a subset of rcon commands", len(Admins), len(Users))

	return cfg, nil
}
