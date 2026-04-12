package environment

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration values loaded from the environment.
type Config struct {
	// MySQL
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisAddr          string
	RedisPassword      string // optional — empty means no AUTH
	RedisDatabaseIndex int

	// JWT
	JWTSecret string

	// Server
	ServerPort string
}

// Load reads the .env file from the working directory (if present), then
// reads all required variables from the process environment. Returns a
// populated *Config or an error listing every missing variable.
func Load() (*Config, error) {
	// Ignore missing .env file — production supplies vars via OS env.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("environment: loading .env file: %w", err)
	}

	cfg := &Config{}
	var missing []string

	required := []struct {
		key string
		ptr *string
	}{
		{"DB_HOST", &cfg.DBHost},
		{"DB_PORT", &cfg.DBPort},
		{"DB_USER", &cfg.DBUser},
		{"DB_PASSWORD", &cfg.DBPassword},
		{"DB_NAME", &cfg.DBName},
		{"REDIS_ADDR", &cfg.RedisAddr},
		{"JWT_SECRET", &cfg.JWTSecret},
		{"SERVER_PORT", &cfg.ServerPort},
	}

	for _, r := range required {
		val := os.Getenv(r.key)
		if val == "" {
			missing = append(missing, r.key)
			continue
		}
		*r.ptr = val
	}

	// REDIS_PASSWORD is optional — an empty string means no Redis AUTH.
	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")

	// REDIS_DB is numeric; require the key, parse as int.
	redisDB := os.Getenv("REDIS_DB")
	if redisDB == "" {
		missing = append(missing, "REDIS_DB")
	} else {
		n, err := strconv.Atoi(redisDB)
		if err != nil {
			return nil, fmt.Errorf("environment: REDIS_DB must be an integer, got %q", redisDB)
		}
		cfg.RedisDatabaseIndex = n
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("environment: missing required variables: %v", missing)
	}

	return cfg, nil
}
