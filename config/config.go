package config

import (
	"os"
	"path"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

const (
	StatusOnline     int8 = iota
	StatusOffline         = 2
	StatusNoNewSyncs      = 3
)

var VERSION string

type ServiceStatus int8

type SystemConfig struct {
	Type   string
	Config map[string]interface{}
}

type Config struct {
	Debug bool
	Log   string

	Systems []SystemConfig
}

func Load() (Config, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return Config{}, err
	}

	viper.SetDefault("Debug", "true")
	viper.SetDefault("Log", path.Join(cacheDir, "gobbs.log"))

	viper.SetConfigName("gobbs")
	viper.SetConfigType("toml")
	viper.AddConfigPath(cfgDir)
	viper.AddConfigPath(homeDir)

	viper.SetEnvPrefix("gobbs")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, err
		} else {
			return Config{}, nil
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (cfg *Config) Save() error {
	cfgFile := viper.ConfigFileUsed()
	if cfgFile == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		cfgFile = path.Join(cfgDir, "gobbs.toml")
	}

	fd, err := os.OpenFile(cfgFile, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer fd.Close()

	if err := toml.NewEncoder(fd).Encode(cfg); err != nil {
		return err
	}

	return nil
}
