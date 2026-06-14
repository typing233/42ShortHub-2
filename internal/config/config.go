package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	App       AppConfig
	Analytics AnalyticsConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
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
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret     string
	ExpireHour int
}

type AppConfig struct {
	BaseURL         string
	ShortCodeLen    int
	RateLimitPerMin int
	MaxBatchSize    int
	BatchWorkers    int
	BatchQueueSize  int
}

type AnalyticsConfig struct {
	GeoIPDBPath       string
	DedupWindowSec    int
	MVRefreshInterval time.Duration
	APIKeyQuotaDaily  int64
	APIKeyRatePerMin  int
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "shortlink"),
			Password: getEnv("DB_PASSWORD", "shortlink"),
			DBName:   getEnv("DB_NAME", "shortlink"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-me-in-production"),
			ExpireHour: getEnvInt("JWT_EXPIRE_HOUR", 72),
		},
		App: AppConfig{
			BaseURL:         getEnv("APP_BASE_URL", "http://localhost:8080"),
			ShortCodeLen:    getEnvInt("SHORT_CODE_LEN", 6),
			RateLimitPerMin: getEnvInt("RATE_LIMIT_PER_MIN", 60),
			MaxBatchSize:    getEnvInt("MAX_BATCH_SIZE", 50),
			BatchWorkers:    getEnvInt("BATCH_WORKERS", 4),
			BatchQueueSize:  getEnvInt("BATCH_QUEUE_SIZE", 100),
		},
		Analytics: AnalyticsConfig{
			GeoIPDBPath:       getEnv("GEOIP_DB_PATH", ""),
			DedupWindowSec:    getEnvInt("DEDUP_WINDOW_SECONDS", 1800),
			MVRefreshInterval: time.Duration(getEnvInt("MV_REFRESH_MINUTES", 5)) * time.Minute,
			APIKeyQuotaDaily:  int64(getEnvInt("API_KEY_DAILY_QUOTA", 1000)),
			APIKeyRatePerMin:  getEnvInt("API_KEY_RATE_PER_MIN", 60),
		},
	}
}

func (d DatabaseConfig) DSN() string {
	return "host=" + d.Host +
		" port=" + d.Port +
		" user=" + d.User +
		" password=" + d.Password +
		" dbname=" + d.DBName +
		" sslmode=" + d.SSLMode
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
