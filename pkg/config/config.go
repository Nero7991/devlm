package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	DatabaseURL      string `json:"database_url"`
	RedisURL         string `json:"redis_url"`
	LLMServiceURL    string `json:"llm_service_url"`
	SandboxImageName string `json:"sandbox_image_name"`
	APIPort          int    `json:"api_port"`
	LogLevel         string `json:"log_level"`
}

var (
	config     *Config
	configOnce sync.Once
	configLock sync.RWMutex
	cache      = make(map[string]interface{})
	cacheTTL   = 5 * time.Minute
)

func LoadConfig(filename string) error {
	var err error
	configOnce.Do(func() {
		file, openErr := os.Open(filename)
		if openErr != nil {
			err = openErr
			return
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		config = &Config{}
		if decodeErr := decoder.Decode(config); decodeErr != nil {
			err = decodeErr
			return
		}

		overrideWithEnv(config)
		err = validateConfig(config)
	})
	return err
}

func overrideWithEnv(cfg *Config) {
	if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
		cfg.DatabaseURL = envURL
	}
	if envURL := os.Getenv("REDIS_URL"); envURL != "" {
		cfg.RedisURL = envURL
	}
	if envURL := os.Getenv("LLM_SERVICE_URL"); envURL != "" {
		cfg.LLMServiceURL = envURL
	}
	if envImage := os.Getenv("SANDBOX_IMAGE_NAME"); envImage != "" {
		cfg.SandboxImageName = envImage
	}
	if envPort := os.Getenv("API_PORT"); envPort != "" {
		if port, convErr := strconv.Atoi(envPort); convErr == nil {
			cfg.APIPort = port
		}
	}
	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		cfg.LogLevel = envLogLevel
	}
}

func GetConfig() (*Config, error) {
	configLock.RLock()
	defer configLock.RUnlock()

	if config == nil {
		return nil, fmt.Errorf("config not loaded. Call LoadConfig() first")
	}
	return config, nil
}

func GetDatabaseURL() (string, error) {
	return getCachedString("database_url", func() (string, error) {
		cfg, err := GetConfig()
		if err != nil {
			return "", err
		}
		return cfg.DatabaseURL, nil
	})
}

func GetRedisURL() (string, error) {
	return getCachedString("redis_url", func() (string, error) {
		cfg, err := GetConfig()
		if err != nil {
			return "", err
		}
		return cfg.RedisURL, nil
	})
}

func GetLLMServiceURL() (string, error) {
	return getCachedString("llm_service_url", func() (string, error) {
		cfg, err := GetConfig()
		if err != nil {
			return "", err
		}
		return cfg.LLMServiceURL, nil
	})
}

func GetSandboxImageName() (string, error) {
	return getCachedString("sandbox_image_name", func() (string, error) {
		cfg, err := GetConfig()
		if err != nil {
			return "", err
		}
		return cfg.SandboxImageName, nil
	})
}

func GetAPIPort() (int, error) {
	return getCachedInt("api_port", func() (int, error) {
		cfg, err := GetConfig()
		if err != nil {
			return 0, err
		}
		return cfg.APIPort, nil
	})
}

func GetLogLevel() (string, error) {
	return getCachedString("log_level", func() (string, error) {
		cfg, err := GetConfig()
		if err != nil {
			return "", err
		}
		return cfg.LogLevel, nil
	})
}

func validateConfig(cfg *Config) error {
	if _, err := url.Parse(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("invalid database URL: %w", err)
	}
	if _, err := url.Parse(cfg.RedisURL); err != nil {
		return fmt.Errorf("invalid Redis URL: %w", err)
	}
	if _, err := url.Parse(cfg.LLMServiceURL); err != nil {
		return fmt.Errorf("invalid LLM service URL: %w", err)
	}
	if cfg.SandboxImageName == "" {
		return fmt.Errorf("sandbox image name cannot be empty")
	}
	if cfg.APIPort < 1 || cfg.APIPort > 65535 {
		return fmt.Errorf("invalid API port: %d", cfg.APIPort)
	}
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLogLevels, strings.ToLower(cfg.LogLevel)) {
		return fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getCachedString(key string, getter func() (string, error)) (string, error) {
	configLock.RLock()
	if val, ok := cache[key]; ok {
		if time.Since(val.(cacheEntry).timestamp) < cacheTTL {
			configLock.RUnlock()
			return val.(cacheEntry).value.(string), nil
		}
	}
	configLock.RUnlock()

	configLock.Lock()
	defer configLock.Unlock()

	value, err := getter()
	if err != nil {
		return "", err
	}

	cache[key] = cacheEntry{value: value, timestamp: time.Now()}
	return value, nil
}

func getCachedInt(key string, getter func() (int, error)) (int, error) {
	configLock.RLock()
	if val, ok := cache[key]; ok {
		if time.Since(val.(cacheEntry).timestamp) < cacheTTL {
			configLock.RUnlock()
			return val.(cacheEntry).value.(int), nil
		}
	}
	configLock.RUnlock()

	configLock.Lock()
	defer configLock.Unlock()

	value, err := getter()
	if err != nil {
		return 0, err
	}

	cache[key] = cacheEntry{value: value, timestamp: time.Now()}
	return value, nil
}

type cacheEntry struct {
	value     interface{}
	timestamp time.Time
}