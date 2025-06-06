package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"github.com/timewave/vault-indexer/go-indexer/logger"
)

type Config struct {
	Ethereum  EthereumConfig   `mapstructure:"ethereum"`
	Contracts []ContractConfig `mapstructure:"contracts"`
	Database  DatabaseConfig   `mapstructure:"database"`
	logger    *logger.Logger
}

type EthereumConfig struct {
	WebsocketURL string
	ChainID      int64 `mapstructure:"chain_id"`
}

type ContractConfig struct {
	Name       string   `mapstructure:"name"`
	Address    string   `mapstructure:"address"`
	StartBlock uint64   `mapstructure:"start_block"`
	EndBlock   uint64   `mapstructure:"end_block"`
	ABIPath    string   `mapstructure:"abi_path"`
	Events     []string `mapstructure:"events"`
}

type DatabaseConfig struct {
	SupabaseURL              string
	SupabaseKey              string
	PostgresConnectionString string
}

func (c *Config) loadEnvFile(env string) error {
	envFile := fmt.Sprintf(".env.%s", env)
	c.logger.Printf("Loading env file from %s", envFile)
	if err := godotenv.Load(envFile); err != nil {
		return fmt.Errorf("failed to load env file: %w", err)
	}
	return nil
}

func New(logger *logger.Logger) *Config {
	return &Config{
		logger: logger,
	}
}

func Load(c *Config) (*Config, error) {

	env := os.Getenv("ENV")
	if env == "" {
		env = "dev" // Default to dev environment
	}
	c.logger.Printf("Loading config for env: %s", env)

	// Load environment variables from .env.{env} file
	if err := c.loadEnvFile(env); err != nil {
		return nil, err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Get the directory of the current file
	_, currentFile, _, _ := runtime.Caller(0)
	configDir := filepath.Dir(currentFile)

	c.logger.Printf("Loading indexer config from: %s", configDir)

	// Add the config directory to the path
	viper.AddConfigPath(configDir)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	c.logger.Printf("Loaded config file from: %s", viper.ConfigFileUsed())

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set credentials from environment variables
	config.Database.SupabaseURL = os.Getenv("INDEXER_SUPABASE_URL")
	config.Database.SupabaseKey = os.Getenv("INDEXER_SUPABASE_ADMIN_KEY")
	config.Ethereum.WebsocketURL = os.Getenv("INDEXER_ETH_RPC_WS_URL")
	config.Database.PostgresConnectionString = os.Getenv("INDEXER_POSTGRES_CONNECTION_STRING")

	return &config, nil
}
