package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	App    AppConfig
	DB     DBConfig
	Redis  RedisConfig
	Worker WorkerConfig
	CORS   CORSConfig
}

type AppConfig struct {
	Env       string
	Version   string
	Port      int
	LogLevel  string
	JWTSecret string
}

type DBConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	DSN      string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type WorkerConfig struct {
	PoolSize int
}

type CORSConfig struct {
	Origins string
}

func Load() (*Config, error) {
	// Load .env file if present (ignore error — env vars may be set directly)
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Env:       getEnv("APP_ENV", "development"),
			Version:   getEnv("APP_VERSION", "0.1.0"),
			Port:      getEnvInt("BACKEND_PORT", 8080),
			LogLevel:  getEnv("LOG_LEVEL", "info"),
			JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 3306),
			Name:     getEnv("DB_NAME", "trader"),
			User:     getEnv("DB_USER", "trader"),
			Password: getEnv("DB_PASSWORD", "traderpassword"),
			DSN:      getEnv("DB_DSN", ""),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Worker: WorkerConfig{
			PoolSize: getEnvInt("WORKER_POOL_SIZE", 10),
		},
		CORS: CORSConfig{
			Origins: getEnv("CORS_ORIGINS", "http://localhost:5173"),
		},
	}

	// Build DSN if not explicitly provided
	if cfg.DB.DSN == "" {
		cfg.DB.DSN = fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
			cfg.DB.User,
			cfg.DB.Password,
			cfg.DB.Host,
			cfg.DB.Port,
			cfg.DB.Name,
		)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}
