package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	SabreEndpoint  string
	SabreUsername  string
	SabrePassword  string
	SabrePCC       string
	SabreDomain    string

	MysearchURL         string
	MysearchPollSeconds int
	SabreRetryMinutes   int
	SabrePollMinutes    int

	BimanGraphqlURL string
}

func LoadConfig() *Config {
	loadDotEnv()

	return &Config{
		SabreEndpoint:  getEnv("SABRE_ENDPOINT", ""),
		SabreUsername:  getEnv("SABRE_USERNAME", ""),
		SabrePassword:  getEnv("SABRE_PASSWORD", ""),
		SabrePCC:       getEnv("SABRE_PCC", ""),
		SabreDomain:    getEnv("SABRE_DOMAIN", "DEFAULT"),
		MysearchURL:         getEnv("MYSEARCH_API_URL", "http://localhost:8000/api/check-seat"),
		MysearchPollSeconds: getEnvInt("MYSEARCH_POLL_INTERVAL", 5),
		SabreRetryMinutes:   getEnvInt("SABRE_RETRY_MINUTES", 20),
		SabrePollMinutes:    getEnvInt("SABRE_POLL_MINUTES", 10),

		BimanGraphqlURL: getEnv("BIMAN_GRAPHQL_URL", "https://booking.biman-airlines.com/api/graphql"),
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

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
