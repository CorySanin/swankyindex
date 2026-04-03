package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

type Conf struct {
	Port          *int    `yaml:"port"`
	Storage       *string `yaml:"storage"`
	Directory     *string `yaml:"directory"`
	HideDownloads *bool   `yaml:"hideDownloads"`
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
		Storage:       PopulateKey(cfg.Storage, "STORAGE", "downloadcount.db"),
		Port:          PopulateKeyInt(cfg.Port, "PORT", 8080),
		Directory:     PopulateKey(cfg.Directory, "DIRECTORY", "/srv/http/"),
		HideDownloads: PopulateKeyBool(cfg.HideDownloads, "HIDEDOWNLOADS", false),
	}
}

func envOrDefault(key string, fallback string) string {
	val, ok := os.LookupEnv(key)
	if ok {
		return val
	}
	return fallback
}

func PopulateKey(fileCfg *string, envKey string, fallback string) *string {
	val, ok := os.LookupEnv(envKey)
	if ok {
		return &val
	}
	if fileCfg != nil {
		return fileCfg
	}
	return &fallback
}

func PopulateKeyInt(fileCfg *int, envKey string, fallback int) *int {
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

func PopulateKeyBool(fileCfg *bool, envKey string, fallback bool) *bool {
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
