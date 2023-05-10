package config

import (
	"fmt"

	"github.com/caarlos0/env"
)

type Config struct {
	HTTP_port string `env:"HTTP_PORT" envDefault:"9000"`
}

var config Config = Config{}

func GetConfig() (*Config, error) {
	if err := env.Parse(&config); err != nil {
		return nil, fmt.Errorf("read logger configuration failed: %w", err)
	}
	return &config, nil
}
