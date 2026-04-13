package config

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

type Conf struct {
	Port          *int    `yaml:"port"`
	Title         *string `yaml:"title"`
	Storage       *string `yaml:"storage"`
	Directory     *string `yaml:"directory"`
	Styles        *string `yaml:"styles"`
	Icons         *bool   `yaml:"icons"`
	HideDownloads *bool   `yaml:"hideDownloads"`
	HideDotfiles  *bool   `yaml:"hideDotfiles"`
	Heading       *string `yaml:"heading"`
	Footer        *string `yaml:"footer"`
	directory     string
}

type expectedType interface {
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
		Title:         populateKey(cfg.Title, "TITLE", new(string("Index of "))),
		Storage:       populateKey(cfg.Storage, "STORAGE", new(string(filepath.Join("data", "downloadcount.db")))),
		Port:          populateKey(cfg.Port, "PORT", new(int(8080))),
		Directory:     NormalizePath(*populateKey(cfg.Directory, "DIRECTORY", new(string("/srv/http/")))),
		Icons:         populateKey(cfg.Icons, "ICONS", new(bool(true))),
		HideDownloads: populateKey(cfg.HideDownloads, "HIDEDOWNLOADS", new(bool(false))),
		HideDotfiles:  populateKey(cfg.HideDotfiles, "HIDEDOTFILES", new(bool(true))),
		Styles:        populateKey(cfg.Styles, "STYLES", new(string("styles.css"))),
		Heading:       populateKey(cfg.Heading, "HEADING", new(string("<h1>Index of <span id=\"path\">%path%</span></h1>"))),
		Footer:        populateKey(cfg.Footer, "FOOTER", new(string(""))),
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

func populateKey[T expectedType](fileCfg *T, envKey string, fallback *T) *T {
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
