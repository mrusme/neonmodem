package config

import (
	"strings"

	"github.com/spf13/viper"
)

const (
	StatusOnline     int8 = iota
	StatusOffline         = 2
	StatusNoNewSyncs      = 3
)

var VERSION string

type ServiceStatus int8

type Config struct {
	Debug string

	Systems []struct {
		Type   string
		Config map[string]interface{}
	}
}

func Load() (Config, error) {
	viper.SetDefault("Debug", "true")

	viper.SetConfigName("gobbs.toml")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/etc/")
	viper.AddConfigPath("$XDG_CONFIG_HOME/")
	viper.AddConfigPath("$HOME/.config/")
	viper.AddConfigPath("$HOME/")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("gobbs")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}
