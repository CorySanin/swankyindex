package config

import (
	"errors"
	"log"
	"os"
	"path"
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
	Heading       *string `yaml:"heading"`
	Footer        *string `yaml:"footer"`
	directory     string
}

func Config() Conf {
	cfg := Conf{}
	file, err := os.ReadFile(envOrDefault("CONFIG", "config.yml"))
	if err != nil {
		log.Print(err)
	} else if err := yaml.Unmarshal(file, &cfg); err != nil {
		log.Print(err)
	}
	return Conf{
		Title:         populateKey(cfg.Title, "TITLE", new(string("Index of "))),
		Storage:       populateKey(cfg.Storage, "STORAGE", new(string("downloadcount.db"))),
		Port:          populateKeyInt(cfg.Port, "PORT", new(int(8080))),
		Directory:     populateKey(cfg.Directory, "DIRECTORY", new(string("/srv/http/"))),
		Icons:         populateKeyBool(cfg.Icons, "ICONS", new(bool(true))),
		HideDownloads: populateKeyBool(cfg.HideDownloads, "HIDEDOWNLOADS", new(bool(false))),
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
		cfg.directory = path.Join(*cfg.Directory)
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

func populateKey(fileCfg *string, envKey string, fallback *string) *string {
	val, ok := os.LookupEnv(envKey)
	if ok {
		return &val
	}
	if fileCfg != nil {
		return fileCfg
	}
	return fallback
}

func populateKeyInt(fileCfg *int, envKey string, fallback *int) *int {
	val, ok := os.LookupEnv(envKey)
	if ok {
		i, err := strconv.Atoi(val)
		if err == nil {
			return &i
		}
	}
	if fileCfg != nil {
		return fileCfg
	}
	return fallback
}

func populateKeyBool(fileCfg *bool, envKey string, fallback *bool) *bool {
	val, ok := os.LookupEnv(envKey)
	if ok && val != "" {
		var result = strings.ToLower(val) == "true" || val == "1"
		return &result
	}
	if fileCfg != nil {
		return fileCfg
	}
	return fallback
}
