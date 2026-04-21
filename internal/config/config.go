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
	Port               *int             `yaml:"port"`
	PrometheusPort     *int             `yaml:"prometheusPort"`
	PrometheusPath     *string          `yaml:"prometheusPath"`
	Title              *string          `yaml:"title"`
	Storage            *string          `yaml:"storage"`
	Directory          *string          `yaml:"directory"`
	Styles             *string          `yaml:"styles"`
	Icons              *bool            `yaml:"icons"`
	ShowDownloads      *bool            `yaml:"showDownloads"`
	ShowDotfiles       *bool            `yaml:"showDotfiles"`
	ShowSymlinks       *bool            `yaml:"showSymlinks"`
	EnableJS           *bool            `yaml:"enableJs"`
	EnableZipDownloads *bool            `yaml:"enableZipDownloads"`
	Heading            *string          `yaml:"heading"`
	Footer             *string          `yaml:"footer"`
	Ignore             []*regexp.Regexp `yaml:"ignore"`
	directory          string
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
		Title:              populatePrimitive(cfg.Title, "TITLE", new(string("Index of "))),
		Storage:            populatePrimitive(cfg.Storage, "STORAGE", new(string(filepath.Join("data", "downloadcount.db")))),
		Port:               populatePrimitive(cfg.Port, "PORT", new(int(8080))),
		PrometheusPort:     populatePrimitive(cfg.PrometheusPort, "PROMPORT", new(int(-1))),
		PrometheusPath:     populatePrimitive(cfg.PrometheusPath, "PROMPATH", new(string("/metrics"))),
		Directory:          NormalizePath(*populatePrimitive(cfg.Directory, "DIRECTORY", new(string("/srv/http/")))),
		Icons:              populatePrimitive(cfg.Icons, "ICONS", new(bool(true))),
		ShowDownloads:      populatePrimitive(cfg.ShowDownloads, "SHOWDOWNLOADS", new(bool(true))),
		ShowDotfiles:       populatePrimitive(cfg.ShowDotfiles, "SHOWDOTFILES", new(bool(false))),
		ShowSymlinks:       populatePrimitive(cfg.ShowSymlinks, "SHOWSYMLINKS", new(bool(true))),
		EnableJS:           populatePrimitive(cfg.EnableJS, "ENABLEJS", new(bool(true))),
		EnableZipDownloads: populatePrimitive(cfg.EnableZipDownloads, "ENABLEZIPDOWNLOADS", new(bool(false))),
		Styles:             populatePrimitive(cfg.Styles, "STYLES", new(string("styles.css"))),
		Heading:            populatePrimitive(cfg.Heading, "HEADING", new(string("<h1>Index of <span id=\"path\">%path%</span></h1>"))),
		Footer:             populatePrimitive(cfg.Footer, "FOOTER", new(string(""))),
		Ignore:             cfg.Ignore,
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
