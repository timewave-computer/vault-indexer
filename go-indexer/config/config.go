package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Ethereum  EthereumConfig   `mapstructure:"ethereum"`
	Contracts []ContractConfig `mapstructure:"contracts"`
	Database  DatabaseConfig   `mapstructure:"database"`
}

type EthereumConfig struct {
	WebsocketURL string `mapstructure:"websocket_url"`
	ChainID      int64  `mapstructure:"chain_id"`
}

type ContractConfig struct {
	Name       string   `mapstructure:"name"`
	Address    string   `mapstructure:"address"`
	StartBlock uint64   `mapstructure:"start_block"`
	ABIPath    string   `mapstructure:"abi_path"`
	Events     []string `mapstructure:"events"`
}

type DatabaseConfig struct {
	SupabaseURL string `mapstructure:"supabase_url"`
	SupabaseKey string `mapstructure:"supabase_admin_key"`
}

func Load() (*Config, error) {
	env := os.Getenv("ENV")
	if env == "" {
		env = "dev" // Default to dev environment
	}

	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yaml")
	viper.AddConfigPath("config")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
