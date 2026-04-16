package config

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

type Conf struct {
	Port          *int             `yaml:"port"`
	Title         *string          `yaml:"title"`
	Storage       *string          `yaml:"storage"`
	Directory     *string          `yaml:"directory"`
	Styles        *string          `yaml:"styles"`
	Icons         *bool            `yaml:"icons"`
	HideDownloads *bool            `yaml:"hideDownloads"`
	HideDotfiles  *bool            `yaml:"hideDotfiles"`
	HideSymlinks  *bool            `yaml:"hideSymlinks"`
	EnableJS      *bool            `yaml:"enableJs"`
	Heading       *string          `yaml:"heading"`
	Footer        *string          `yaml:"footer"`
	Ignore        []*regexp.Regexp `yaml:"ignore"`
	directory     string
}

type expectedPrimitive interface {
	bool | int | string
}

const (
	SeparatorString = string(os.PathSeparator)
)

func Config() Conf {
	cfg := Conf{}
	file, err := os.ReadFile(envOrDefault("CONFIG", filepath.Join("data", "config.yml")))
	if err != nil {
		log.Print(err)
	} else if err := yaml.Unmarshal(file, &cfg); err != nil {
		log.Print(err)
	}
	return Conf{
		Title:         populatePrimitive(cfg.Title, "TITLE", new(string("Index of "))),
		Storage:       populatePrimitive(cfg.Storage, "STORAGE", new(string(filepath.Join("data", "downloadcount.db")))),
		Port:          populatePrimitive(cfg.Port, "PORT", new(int(8080))),
		Directory:     NormalizePath(*populatePrimitive(cfg.Directory, "DIRECTORY", new(string("/srv/http/")))),
		Icons:         populatePrimitive(cfg.Icons, "ICONS", new(bool(true))),
		HideDownloads: populatePrimitive(cfg.HideDownloads, "HIDEDOWNLOADS", new(bool(false))),
		HideDotfiles:  populatePrimitive(cfg.HideDotfiles, "HIDEDOTFILES", new(bool(true))),
		HideSymlinks:  populatePrimitive(cfg.HideSymlinks, "HIDESYMLINKS", new(bool(false))),
		EnableJS:      populatePrimitive(cfg.EnableJS, "ENABLEJS", new(bool(true))),
		Styles:        populatePrimitive(cfg.Styles, "STYLES", new(string("styles.css"))),
		Heading:       populatePrimitive(cfg.Heading, "HEADING", new(string("<h1>Index of <span id=\"path\">%path%</span></h1>"))),
		Footer:        populatePrimitive(cfg.Footer, "FOOTER", new(string(""))),
		Ignore:        cfg.Ignore,
	}
}

func (cfg *Conf) GetDirectory() (string, error) {
	if cfg.Directory == nil {
		return "", errors.New("No directory set in config")
	}
	if cfg.directory == "" {
		cfg.directory = filepath.Join(*cfg.Directory)
	}
	return cfg.directory, nil
}

func envOrDefault(key string, fallback string) string {
	val, ok := os.LookupEnv(key)
	if ok {
		return val
	}
	return fallback
}

func populatePrimitive[T expectedPrimitive](fileCfg *T, envKey string, fallback *T) *T {
	val, ok := os.LookupEnv(envKey)
	if ok {
		switch any(*fallback).(type) {
		case string:
			if typed, ok := any(val).(T); ok {
				return &typed
			}
		case bool:
			result := strings.ToLower(val) == "true" || val == "1"
			if typed, ok := any(result).(T); ok && val != "" {
				return &typed
			}
		case int:
			i, err := strconv.Atoi(val)
			if err != nil {
				break
			}
			if typed, ok := any(i).(T); ok && val != "" {
				return &typed
			}
		default:
			break
		}
	}
	if fileCfg != nil {
		return fileCfg
	}
	return fallback
}

func NormalizePath(p string) *string {
	return new(string(strings.TrimSuffix(p, SeparatorString) + SeparatorString))
}
