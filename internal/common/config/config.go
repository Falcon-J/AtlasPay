package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Development        bool
	CORSAllowedOrigins []string
	Server             ServerConfig
	Database           DatabaseConfig
	Redis              RedisConfig
	Kafka              KafkaConfig
	JWT                JWTConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	RateLimit    int
	RateBurst    int
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type KafkaConfig struct {
	Brokers []string
	GroupID string
	Enabled bool
	Workers int
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// Load returns configuration from environment variables
func Load() *Config {
	development := os.Getenv("APP_ENV") == "dev"
	accessDefault, refreshDefault := "", ""
	if development {
		accessDefault = "atlaspay-access-secret-change-in-prod"
		refreshDefault = "atlaspay-refresh-secret-change-in-prod"
	}
	return &Config{
		Development:        development,
		CORSAllowedOrigins: strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ","),
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
			RateLimit:    getIntEnv("RATE_LIMIT_REQUESTS", 100),
			RateBurst:    getIntEnv("RATE_LIMIT_BURST", 10),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "atlaspay"),
			Password: getEnv("DB_PASSWORD", "atlaspay_secret"),
			DBName:   getEnv("DB_NAME", "atlaspay"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "127.0.0.1"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getIntEnv("REDIS_DB", 0),
		},
		Kafka: KafkaConfig{
			Brokers: []string{getEnv("KAFKA_BROKERS", "127.0.0.1:9092")},
			GroupID: getEnv("KAFKA_GROUP_ID", "atlaspay"),
			Enabled: getBoolEnv("KAFKA_ENABLED", false),
			Workers: getIntEnv("KAFKA_WORKERS", 1),
		},
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", accessDefault),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", refreshDefault),
			AccessExpiry:  getDurationEnv("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshExpiry: getDurationEnv("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
	}
}

// Validate checks security configuration before opening listeners or connecting dependencies.
func (c *Config) Validate(service string) error {
	origins := make([]string, 0, len(c.CORSAllowedOrigins))
	for _, raw := range c.CORSAllowedOrigins {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			continue
		}
		if origin == "*" && c.Development {
			origins = append(origins, origin)
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(origin, "*") || origin != u.Scheme+"://"+u.Host {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS must contain exact HTTP or HTTPS origins")
		}
		origins = append(origins, origin)
	}
	c.CORSAllowedOrigins = origins
	if c.Development {
		return nil
	}
	if service == "api-gateway" {
		for _, secret := range []string{c.JWT.AccessSecret, c.JWT.RefreshSecret} {
			if len(secret) < 32 || strings.TrimSpace(secret) == "" {
				return fmt.Errorf("JWT signing secrets must be non-default values of at least 32 bytes")
			}
			switch secret {
			case "atlaspay-access-secret-change-in-prod", "atlaspay-refresh-secret-change-in-prod",
				"atlaspay-access-secret-dev-only", "atlaspay-refresh-secret-dev-only",
				"atlaspay-access-secret-prod-change-me", "atlaspay-refresh-secret-prod-change-me":
				return fmt.Errorf("JWT signing secrets must be non-default values of at least 32 bytes")
			}
		}
		if c.JWT.AccessSecret == c.JWT.RefreshSecret {
			return fmt.Errorf("JWT access and refresh signing secrets must differ")
		}
	} else {
		key := strings.ToUpper(strings.TrimSuffix(service, "-service")) + "_SERVICE_TOKEN"
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return fmt.Errorf("%s is required outside development", key)
		}
	}
	for _, name := range []string{"PAYMENT", "INVENTORY", "ORDER"} {
		if os.Getenv(name+"_SERVICE_URL") != "" && strings.TrimSpace(os.Getenv(name+"_SERVICE_TOKEN")) == "" {
			return fmt.Errorf("%s_SERVICE_TOKEN is required for configured outbound service", name)
		}
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// DatabaseURL returns the PostgreSQL connection string
func (c *DatabaseConfig) DatabaseURL() string {
	return "postgres://" + c.User + ":" + c.Password + "@" + c.Host + ":" + c.Port + "/" + c.DBName + "?sslmode=" + c.SSLMode
}

// RedisAddr returns the Redis address
func (r *RedisConfig) RedisAddr() string {
	return r.Host + ":" + r.Port
}
