package config

import (
	"errors"
	"html/template"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

type Conf struct {
	Port          *int    `yaml:"port"`
	Storage       *string `yaml:"storage"`
	Directory     *string `yaml:"directory"`
	HideDownloads *bool   `yaml:"hideDownloads"`
	heading       string  `yaml:"heading"`
	footer        string  `yaml:"footer"`
	Heading       template.HTML
	Footer        template.HTML
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
		Storage:       populateKey(cfg.Storage, "STORAGE", "downloadcount.db"),
		Port:          populateKeyInt(cfg.Port, "PORT", 8080),
		Directory:     populateKey(cfg.Directory, "DIRECTORY", "/srv/http/"),
		HideDownloads: populateKeyBool(cfg.HideDownloads, "HIDEDOWNLOADS", false),
		Heading:       template.HTML(cfg.Heading),
		Footer:        template.HTML(cfg.Footer),
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

func populateKey(fileCfg *string, envKey string, fallback string) *string {
	val, ok := os.LookupEnv(envKey)
	if ok {
		return &val
	}
	if fileCfg != nil {
		return fileCfg
	}
	return &fallback
}

func populateKeyInt(fileCfg *int, envKey string, fallback int) *int {
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
	return &fallback
}

func populateKeyBool(fileCfg *bool, envKey string, fallback bool) *bool {
	val, ok := os.LookupEnv(envKey)
	if ok && val != "" {
		var result = strings.ToLower(val) == "true" || val == "1"
		return &result
	}
	if fileCfg != nil {
		return fileCfg
	}
	return &fallback
}
