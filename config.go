package main

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	SabreEndpoint string
	SabreUsername string
	SabrePassword string
	SabrePCC      string
	SabreDomain   string
	IsLive        bool
}

func LoadConfig() *Config {
	loadDotEnv()

	return &Config{
		SabreEndpoint: getEnv("SABRE_ENDPOINT", "https://webservices.platform.sabre.com"),
		SabreUsername: getEnv("SABRE_USERNAME", "593716"),
		SabrePassword: getEnv("SABRE_PASSWORD", "83015s3p"),
		SabrePCC:      getEnv("SABRE_PCC", "06EL"),
		SabreDomain:   getEnv("SABRE_DOMAIN", "DEFAULT"),
		IsLive:        getEnvBool("IS_LIVE", true),
	}
}

func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Remove surrounding quotes if any
		val = strings.Trim(val, "\"'")
		if key != "" {
			os.Setenv(key, val)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return fallback
}
