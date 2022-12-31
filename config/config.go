package config

import (
	"os"
	"path"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

type ThemeItemConfig struct {
	Foreground lipgloss.AdaptiveColor
	Background lipgloss.AdaptiveColor
	Border     struct {
		Foreground lipgloss.AdaptiveColor
		Background lipgloss.AdaptiveColor
		Border     lipgloss.Border
		Sides      []bool
	}
	Padding []int
	Margin  []int
}

type Config struct {
	Debug bool
	Log   string

	Systems []SystemConfig

	Theme struct {
		DialogBox struct {
			Window    ThemeItemConfig
			Titlebar  ThemeItemConfig
			Bottombar ThemeItemConfig
		}

		PostsList struct {
			List struct {
				Focused ThemeItemConfig
				Blurred ThemeItemConfig
			}
			Item struct {
				Focused  ThemeItemConfig
				Blurred  ThemeItemConfig
				Selected ThemeItemConfig
			}
			ItemDetail struct {
				Focused  ThemeItemConfig
				Blurred  ThemeItemConfig
				Selected ThemeItemConfig
			}
		}

		Post struct {
			Author  ThemeItemConfig
			Subject ThemeItemConfig
		}

		Reply struct {
			Author ThemeItemConfig
		}
	}
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

	// PostsList List:Focused
	viper.SetDefault("Theme.PostsList.List.Focused.Margin",
		[]int{0, 0, 0, 0})
	viper.SetDefault("Theme.PostsList.List.Focused.Padding",
		[]int{1, 1, 1, 1})
	viper.SetDefault("Theme.PostsList.List.Focused.Border.Border",
		lipgloss.RoundedBorder())
	viper.SetDefault("Theme.PostsList.List.Focused.Border.Sides",
		[]bool{true, true, true, true},
	)
	viper.SetDefault("Theme.PostsList.List.Focused.Border.Foreground",
		lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"})

	// PostsList List:Blurred
	viper.SetDefault("Theme.PostsList.List.Blurred.Margin",
		[]int{0, 0, 0, 0})
	viper.SetDefault("Theme.PostsList.List.Blurred.Padding",
		[]int{1, 1, 1, 1})
	viper.SetDefault("Theme.PostsList.List.Blurred.Border.Border",
		lipgloss.RoundedBorder())
	viper.SetDefault("Theme.PostsList.List.Blurred.Border.Sides",
		[]bool{true, true, true, true},
	)
	viper.SetDefault("Theme.PostsList.List.Blurred.Border.Foreground",
		lipgloss.AdaptiveColor{Light: "#cccccc", Dark: "#333333"})

	// PostsList Item:Focused
	viper.SetDefault("Theme.PostsList.Item.Focused.Padding",
		[]int{0, 0, 0, 2})
	viper.SetDefault("Theme.PostsList.Item.Focused.Foreground",
		lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"})

	// PostsList Item:Blurred
	viper.SetDefault("Theme.PostsList.Item.Blurred.Padding",
		[]int{0, 0, 0, 2})
	viper.SetDefault("Theme.PostsList.Item.Blurred.Foreground",
		lipgloss.AdaptiveColor{Light: "#cccccc", Dark: "#333333"})

	// PostsList Item:Selected
	viper.SetDefault("Theme.PostsList.Item.Selected.Padding",
		[]int{0, 0, 0, 1})
	viper.SetDefault("Theme.PostsList.Item.Selected.Border.Border",
		lipgloss.NormalBorder())
	viper.SetDefault("Theme.PostsList.Item.Selected.Border.Sides",
		[]bool{false, false, false, true},
	)
	viper.SetDefault("Theme.PostsList.Item.Selected.Border.Foreground",
		lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})
	viper.SetDefault("Theme.PostsList.Item.Selected.Foreground",
		lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	// PostsList ItemDetail:Focused
	viper.SetDefault("Theme.PostsList.ItemDetail.Focused.Padding",
		[]int{0, 0, 0, 2})
	viper.SetDefault("Theme.PostsList.ItemDetail.Focused.Foreground",
		lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"})

	// PostsList ItemDetail:Blurred
	viper.SetDefault("Theme.PostsList.ItemDetail.Blurred.Padding",
		[]int{0, 0, 0, 2})
	viper.SetDefault("Theme.PostsList.ItemDetail.Blurred.Foreground",
		lipgloss.AdaptiveColor{Light: "#cccccc", Dark: "#333333"})

	// PostsList ItemDetail:Selected
	viper.SetDefault("Theme.PostsList.ItemDetail.Selected.Padding",
		[]int{0, 0, 0, 1})
	viper.SetDefault("Theme.PostsList.ItemDetail.Selected.Border.Border",
		lipgloss.NormalBorder())
	viper.SetDefault("Theme.PostsList.ItemDetail.Selected.Border.Sides",
		[]bool{false, false, false, true},
	)
	viper.SetDefault("Theme.PostsList.ItemDetail.Selected.Border.Foreground",
		lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})
	viper.SetDefault("Theme.PostsList.ItemDetail.Selected.Foreground",
		lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	// DialogBox Window
	viper.SetDefault("Theme.DialogBox.Window.Margin",
		[]int{0, 0, 0, 0})
	viper.SetDefault("Theme.DialogBox.Window.Padding",
		[]int{0, 0, 0, 0})
	viper.SetDefault("Theme.DialogBox.Window.Border.Border",
		lipgloss.ThickBorder())
	viper.SetDefault("Theme.DialogBox.Window.Border.Sides",
		[]bool{false, true, true, true},
	)
	viper.SetDefault("Theme.DialogBox.Window.Border.Foreground",
		lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"})

	// DialogBox Titlebar
	viper.SetDefault("Theme.DialogBox.Titlebar.Margin",
		[]int{0, 0, 1, 0})
	viper.SetDefault("Theme.DialogBox.Titlebar.Padding",
		[]int{0, 1, 0, 1})
	viper.SetDefault("Theme.DialogBox.Titlebar.Foreground",
		lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#000000"})
	viper.SetDefault("Theme.DialogBox.Titlebar.Background",
		lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"})

	// DialogBox Bottombar
	viper.SetDefault("Theme.DialogBox.Bottombar.Margin",
		[]int{1, 0, 0, 0})
	viper.SetDefault("Theme.DialogBox.Bottombar.Padding",
		[]int{0, 1, 0, 1})
	viper.SetDefault("Theme.DialogBox.Bottombar.Foreground",
		lipgloss.AdaptiveColor{Light: "#aaaaaa", Dark: "#999999"})

	// Post Author
	viper.SetDefault("Theme.Post.Author.Padding",
		[]int{0, 1, 0, 1})
	viper.SetDefault("Theme.Post.Author.Foreground",
		lipgloss.AdaptiveColor{Light: "#F25D94", Dark: "#F25D94"})

	// Post Subject
	viper.SetDefault("Theme.Post.Subject.Padding",
		[]int{0, 1, 0, 1})
	viper.SetDefault("Theme.Post.Subject.Foreground",
		lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"})
	viper.SetDefault("Theme.Post.Subject.Background",
		lipgloss.AdaptiveColor{Light: "#F25D94", Dark: "#F25D94"})

	// Reply Author
	viper.SetDefault("Theme.Reply.Author.Padding",
		[]int{0, 1, 0, 1})
	viper.SetDefault("Theme.Reply.Author.Foreground",
		lipgloss.AdaptiveColor{Light: "#000000", Dark: "#00000"})
	viper.SetDefault("Theme.Reply.Author.Foreground",
		lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#874BFD"})

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
