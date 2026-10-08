package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/pelletier/go-toml/v2"
)

var VERSION string

const (
	FileName  = "neonmodem.toml"
	envPrefix = "NEONMODEM_"

	DefaultReadTimeout  = 20
	DefaultWriteTimeout = 60
)

type SystemConfig struct {
	Type     string
	Settings system.Settings `toml:"Config"`
}

type Adaptive struct {
	Light string
	Dark  string
}

func (a Adaptive) IsZero() bool {
	return a.Light == "" && a.Dark == ""
}

type Border struct {
	Top          string
	Bottom       string
	Left         string
	Right        string
	TopLeft      string
	TopRight     string
	BottomLeft   string
	BottomRight  string
	MiddleLeft   string
	MiddleRight  string
	Middle       string
	MiddleTop    string
	MiddleBottom string
}

func (b Border) IsZero() bool {
	return b == Border{}
}

type BorderConfig struct {
	Foreground Adaptive
	Background Adaptive
	Border     Border
	Sides      []bool
}

type ThemeItem struct {
	Foreground Adaptive
	Background Adaptive
	Border     BorderConfig
	Padding    []int
	Margin     []int
}

type FocusedBlurred struct {
	Focused ThemeItem
	Blurred ThemeItem
}

type FocusedBlurredSelected struct {
	Focused  ThemeItem
	Blurred  ThemeItem
	Selected ThemeItem
}

type DialogTheme struct {
	Window    FocusedBlurred
	Titlebar  FocusedBlurred
	Bottombar ThemeItem
}

type ListTheme struct {
	List       FocusedBlurred
	Item       FocusedBlurredSelected
	ItemDetail FocusedBlurredSelected
}

type Theme struct {
	Header struct {
		Selector ThemeItem
		Spinner  ThemeItem
	}

	DialogBox      DialogTheme
	ErrorDialogBox DialogTheme

	PostsList ListTheme
	PopupList ListTheme

	Post struct {
		Author  ThemeItem
		Subject ThemeItem
	}

	Reply struct {
		Author ThemeItem
	}
}

type Config struct {
	Debug         bool
	Log           string
	Proxy         string
	Browser       string
	Sort          string
	RenderShadows bool
	RenderImages  bool
	RenderSplash  bool
	RenderBanner  bool
	ReadTimeout   int
	WriteTimeout  int

	Systems []SystemConfig `toml:"Systems,omitempty"`

	OpenWith []OpenWith `toml:"OpenWith,omitempty"`

	Theme Theme

	path     string
	notices  []string
	loaded   *Config
	file     fs.FileInfo
	defaults *Config
}

func (c *Config) Path() string {
	return c.path
}

func Load() (*Config, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}

	cfg, err := LoadFrom(
		[]string{filepath.Join(cfgDir, FileName), filepath.Join(homeDir, FileName)},
		cacheDir,
	)
	if err != nil {
		return nil, err
	}
	if cfg.path == "" {
		cfg.path = filepath.Join(cfgDir, FileName)
	}

	cfg.applyEnv()
	cfg.normalize()

	return cfg, nil
}

func LoadFrom(candidates []string, cacheDir string) (*Config, error) {
	defaults := Defaults(cacheDir)
	cfg := Defaults(cacheDir)
	cfg.defaults = &defaults

	for _, candidate := range candidates {
		data, info, err := readFile(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}

		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", candidate, err)
		}
		cfg.path = candidate
		cfg.file = info
		break
	}
	loaded := cfg
	cfg.loaded = &loaded
	cfg.normalize()

	return &cfg, nil
}

func readFile(path string) ([]byte, fs.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}

	return data, info, nil
}

func (c *Config) applyEnv() {
	if v, ok := lookupEnv("DEBUG"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			c.Debug = b
		}
	}
	if v, ok := lookupEnv("LOG"); ok {
		c.Log = v
	}
	if v, ok := lookupEnv("PROXY"); ok {
		c.Proxy = v
	}
	if v, ok := lookupEnv("BROWSER"); ok {
		c.Browser = v
	}
	if v, ok := lookupEnv("SORT"); ok {
		c.Sort = v
	}
	for name, target := range map[string]*bool{
		"RENDERSHADOWS": &c.RenderShadows,
		"RENDERIMAGES":  &c.RenderImages,
		"RENDERSPLASH":  &c.RenderSplash,
		"RENDERBANNER":  &c.RenderBanner,
	} {
		if v, ok := lookupEnv(name); ok {
			if b, err := strconv.ParseBool(v); err == nil {
				*target = b
			}
		}
	}
	for _, t := range c.timeouts() {
		v, ok := lookupEnv(t.env)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			c.notices = append(c.notices, fmt.Sprintf(
				"%s%s=%q isn't a whole number of seconds; using %d", envPrefix, t.env, v, *t.value))
			continue
		}
		*t.value = n
	}
}

type timeoutSetting struct {
	name     string
	env      string
	value    *int
	fallback int
}

func (c *Config) timeouts() []timeoutSetting {
	return []timeoutSetting{
		{"ReadTimeout", "READTIMEOUT", &c.ReadTimeout, DefaultReadTimeout},
		{"WriteTimeout", "WRITETIMEOUT", &c.WriteTimeout, DefaultWriteTimeout},
	}
}

func (c *Config) normalize() {
	for _, t := range c.timeouts() {
		if *t.value < 0 {
			c.notices = append(c.notices, fmt.Sprintf(
				"The %s setting %d is negative; using %d", t.name, *t.value, t.fallback))
			*t.value = t.fallback
		}
	}
}

func (c *Config) Notices() []string {
	return c.notices
}

func (c *Config) ReadDeadline() time.Duration {
	return time.Duration(c.ReadTimeout) * time.Second
}

func (c *Config) WriteDeadline() time.Duration {
	return time.Duration(c.WriteTimeout) * time.Second
}

func lookupEnv(name string) (string, bool) {
	return os.LookupEnv(envPrefix + name)
}

func (c *Config) Save() error {
	path := c.path
	if path == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		path = filepath.Join(cfgDir, FileName)
	}

	doc, err := c.Document()
	if err != nil {
		return err
	}

	if err := writeInPlace(path, doc); err != nil {
		return err
	}
	c.path = path

	return nil
}

func (c *Config) Document() ([]byte, error) {
	saved := c
	if c.loaded != nil {
		copied := *c.loaded
		copied.Systems = c.Systems
		saved = &copied
	}

	current, err := toMap(saved)
	if err != nil {
		return nil, err
	}

	if c.defaults != nil {
		defaults, err := toMap(c.defaults)
		if err != nil {
			return nil, err
		}
		prune(current, defaults)
	}

	return toml.Marshal(current)
}

func toMap(v any) (map[string]any, error) {
	data, err := toml.Marshal(v)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func prune(current map[string]any, defaults map[string]any) {
	for key, value := range current {
		def, ok := defaults[key]
		if !ok {
			continue
		}

		sub, isMap := value.(map[string]any)
		defSub, defIsMap := def.(map[string]any)
		if isMap && defIsMap {
			prune(sub, defSub)
			if len(sub) == 0 {
				delete(current, key)
			}
			continue
		}

		if reflect.DeepEqual(value, def) {
			delete(current, key)
		}
	}
}

func writeInPlace(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 == 0 {
		return nil
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("the file was written, but its mode couldn't be set to 0600: %w", err)
	}
	return nil
}

func Snippet(entry SystemConfig) (string, error) {
	m, err := toMap(map[string]any{"Systems": []SystemConfig{entry}})
	if err != nil {
		return "", err
	}
	dropEmpty(m)

	data, err := toml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func dropEmpty(m map[string]any) {
	for key, value := range m {
		switch v := value.(type) {
		case map[string]any:
			dropEmpty(v)
			if len(v) == 0 {
				delete(m, key)
			}
		case []any:
			for _, item := range v {
				if sub, ok := item.(map[string]any); ok {
					dropEmpty(sub)
				}
			}
		}
	}
}
