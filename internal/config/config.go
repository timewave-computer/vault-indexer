package config

import (
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
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("config")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
