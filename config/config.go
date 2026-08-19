package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the single source of truth for every tunable value in the app.
// Nothing else in the codebase reads os.Getenv directly.
type Config struct {
	App       AppConfig
	DB        DBConfig
	JWT       JWTConfig
	Auth      AuthConfig
	Mail      MailConfig
	RateLimit RateLimitConfig
	Redis     RedisConfig
}

// RedisConfig configures the cache. Enabled is the switch: when false nothing
// connects and cache.Null is used, so the application behaves identically —
// correct, just without the speed-up.
type RedisConfig struct {
	Enabled  bool
	Host     string
	Port     string
	Username string
	Password string
	DB       int

	// Prefix namespaces every key so one Redis instance can safely back several
	// applications, or several environments.
	Prefix string

	DialTimeout time.Duration
	Timeout     time.Duration
	PoolSize    int

	// DefaultTTL is how long a cached entry lives when the caller does not
	// specify. Short by default: a stale read is a bug report you cannot
	// reproduce, so the cache should forget quickly until proven otherwise.
	DefaultTTL time.Duration

	// ReactionFlushInterval is how often reactions held in Redis are written
	// to MySQL. It is also the size of the window in which a Redis failure
	// loses reactions, so it is short by default.
	ReactionFlushInterval time.Duration

	// PostTTL overrides DefaultTTL for cached posts.
	//
	// Zero means the entries never expire. That is safe only because the post
	// cache is maintained by events rather than by expiry — writing a post
	// files it, editing replaces it, deleting removes it — so nothing depends
	// on a clock to become correct. What expiry still protects against is a
	// change made outside this application, and a listener that never ran
	// because the process died between saving and announcing.
	PostTTL time.Duration
}

// Addr is the host:port the Redis client expects.
func (r RedisConfig) Addr() string { return r.Host + ":" + r.Port }

// RateLimitConfig throttles callers. Two tiers: a general allowance for the
// whole API, and a much tighter one for the auth endpoints, which send mail and
// run bcrypt for unauthenticated callers.
type RateLimitConfig struct {
	Enabled      bool
	Requests     int
	Window       time.Duration
	AuthRequests int
	AuthWindow   time.Duration
}

type AppConfig struct {
	Name            string
	Env             string
	URL             string
	Port            string
	Debug           bool
	ShutdownTimeout time.Duration

	// TrustedProxies lists the CIDRs whose X-Forwarded-For header may be
	// believed. Empty means trust none and use the socket's remote address.
	//
	// This is a security setting, not a convenience one: the rate limiter keys
	// anonymous callers by IP, so if a spoofable header decided that IP, anyone
	// could mint unlimited fresh buckets and the throttle would be decorative.
	TrustedProxies []string

	// CORSAllowedOrigins is empty or ["*"] for any origin.
	CORSAllowedOrigins []string
}

type JWTConfig struct {
	Secret     string
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type AuthConfig struct {
	// RequireEmailVerification is the switch: when false the whole verification
	// flow is inert — no emails, no 403s — and the endpoints still work for
	// anyone who wants to opt in.
	RequireEmailVerification bool
	VerificationTTL          time.Duration

	// PasswordResetTTL is deliberately short — a live reset link is a
	// temporary key to the account.
	PasswordResetTTL time.Duration

	// PasswordResetURL is where the emailed link points. Unlike the
	// verification link, which the API redeems itself via GET, a reset needs a
	// page where the user can type a new password — so this points at your
	// frontend, and that page POSTs the token to /auth/password/reset.
	PasswordResetURL string
}

type MailConfig struct {
	Host        string
	Port        string
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

// Addr is the host:port net/smtp expects.
func (m MailConfig) Addr() string { return m.Host + ":" + m.Port }

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	Charset         string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// DSN builds the MySQL connection string GORM expects.
// parseTime=True is required so DATETIME columns scan into time.Time.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.Name, d.Charset,
	)
}

// Load reads .env (if present) and materialises the config.
// A missing .env is not fatal — real environments inject variables directly.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("config: no .env file found, falling back to environment variables")
	}

	return &Config{
		App: AppConfig{
			Name: env("APP_NAME", "GoBook"),
			// Defaults to production, not local: an unset APP_ENV is far more
			// likely to be a deploy that forgot to set it than a developer who
			// skipped `make setup`. The safe answer costs a developer one line
			// in .env; the convenient answer costs a production system its debug
			// output and its seed guard.
			Env:  env("APP_ENV", "production"),
			URL:  strings.TrimRight(env("APP_URL", "http://localhost:8080"), "/"),
			Port: env("APP_PORT", "8080"),
			// Debug returns panic detail to clients and logs every query, so it
			// defaults on only in development. A deploy that forgets to set it
			// gets the safe behaviour rather than the convenient one.
			Debug:              envBool("APP_DEBUG", isDevelopmentEnv(env("APP_ENV", "production"))),
			ShutdownTimeout:    time.Duration(envInt("APP_SHUTDOWN_TIMEOUT", 10)) * time.Second,
			TrustedProxies:     envList("TRUSTED_PROXIES"),
			CORSAllowedOrigins: envList("CORS_ALLOWED_ORIGINS"),
		},
		RateLimit: RateLimitConfig{
			Enabled:      envBool("RATE_LIMIT_ENABLED", true),
			Requests:     envInt("RATE_LIMIT_REQUESTS", 120),
			Window:       time.Duration(envInt("RATE_LIMIT_WINDOW", 60)) * time.Second,
			AuthRequests: envInt("RATE_LIMIT_AUTH_REQUESTS", 10),
			AuthWindow:   time.Duration(envInt("RATE_LIMIT_AUTH_WINDOW", 60)) * time.Second,
		},
		JWT: JWTConfig{
			Secret:     env("JWT_SECRET", ""),
			Issuer:     env("JWT_ISSUER", "gobook"),
			AccessTTL:  time.Duration(envInt("JWT_ACCESS_TTL", 15)) * time.Minute,
			RefreshTTL: time.Duration(envInt("JWT_REFRESH_TTL", 720)) * time.Hour,
		},
		Auth: AuthConfig{
			RequireEmailVerification: envBool("AUTH_REQUIRE_EMAIL_VERIFICATION", false),
			VerificationTTL:          time.Duration(envInt("AUTH_VERIFICATION_TTL", 24)) * time.Hour,
			PasswordResetTTL:         time.Duration(envInt("AUTH_PASSWORD_RESET_TTL", 60)) * time.Minute,
			PasswordResetURL: strings.TrimRight(env(
				"AUTH_PASSWORD_RESET_URL",
				env("APP_URL", "http://localhost:8080")+"/reset-password",
			), "/"),
		},
		Mail: MailConfig{
			Host:        env("MAIL_HOST", "127.0.0.1"),
			Port:        env("MAIL_PORT", "1025"),
			Username:    env("MAIL_USERNAME", ""),
			Password:    env("MAIL_PASSWORD", ""),
			FromAddress: env("MAIL_FROM_ADDRESS", ""),
			FromName:    env("MAIL_FROM_NAME", "GoBook"),
		},
		Redis: RedisConfig{
			Enabled:     envBool("REDIS_ENABLED", false),
			Host:        env("REDIS_HOST", "127.0.0.1"),
			Port:        env("REDIS_PORT", "6379"),
			Username:    env("REDIS_USERNAME", ""),
			Password:    env("REDIS_PASSWORD", ""),
			DB:          envInt("REDIS_DB", 0),
			Prefix:      env("REDIS_PREFIX", "gobook"),
			DialTimeout: time.Duration(envInt("REDIS_DIAL_TIMEOUT", 5)) * time.Second,
			Timeout:     time.Duration(envInt("REDIS_TIMEOUT", 3)) * time.Second,
			PoolSize:    envInt("REDIS_POOL_SIZE", 10),
			DefaultTTL:  time.Duration(envInt("CACHE_DEFAULT_TTL", 300)) * time.Second,
			// -1 is the "not configured" sentinel, so an explicit 0 can mean
			// "never expire" rather than being mistaken for an absent value.
			PostTTL:               cacheTTL("CACHE_POST_TTL", envInt("CACHE_DEFAULT_TTL", 300)),
			ReactionFlushInterval: time.Duration(envInt("REACTION_FLUSH_INTERVAL", 10)) * time.Second,
		},
		DB: DBConfig{
			Host:            env("DB_HOST", "127.0.0.1"),
			Port:            env("DB_PORT", "3306"),
			User:            env("DB_USER", "root"),
			Password:        env("DB_PASSWORD", ""),
			Name:            env("DB_NAME", "go_mvc"),
			Charset:         env("DB_CHARSET", "utf8mb4"),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 10),
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 100),
			ConnMaxLifetime: time.Duration(envInt("DB_CONN_MAX_LIFETIME", 60)) * time.Minute,
		},
	}
}

// cacheTTL reads a TTL in seconds, falling back to fallbackSeconds when the
// variable is not set. A configured 0 is preserved and means "never expires".
func cacheTTL(key string, fallbackSeconds int) time.Duration {
	seconds := envInt(key, -1)
	if seconds < 0 {
		seconds = fallbackSeconds
	}
	return time.Duration(seconds) * time.Second
}

func (a AppConfig) IsProduction() bool { return a.Env == "production" }

// IsDevelopment reports whether this is an environment where fixture data is
// appropriate. Seeding inserts accounts with a published password, so it is
// gated on an allow-list rather than on "not production" — an unrecognised
// APP_ENV like "staging" or a typo must fail closed, not seed a public admin.
func (a AppConfig) IsDevelopment() bool { return isDevelopmentEnv(a.Env) }

func isDevelopmentEnv(name string) bool {
	switch name {
	case "local", "development", "dev", "test", "testing":
		return true
	default:
		return false
	}
}

// minSecretLen is 32 bytes because that matches the output size of the SHA-256
// HMAC used to sign tokens — a shorter key adds no security.
const minSecretLen = 32

// Validate rejects a configuration the app cannot safely run under. It is
// called by app.New() rather than Load(), so `cmd/migrate` — which needs no
// signing key — is unaffected.
//
// A signing secret that silently falls back to a default is a security hole:
// anyone who has read the source can mint valid tokens. That makes this a hard
// failure at boot, in the same spirit as database.Connect pinging the pool
// before returning.
func (c *Config) Validate() error {
	var problems []string

	switch {
	case c.JWT.Secret == "":
		problems = append(problems, "JWT_SECRET is required (generate one with: openssl rand -base64 48)")
	case len(c.JWT.Secret) < minSecretLen:
		problems = append(problems,
			fmt.Sprintf("JWT_SECRET must be at least %d characters, got %d", minSecretLen, len(c.JWT.Secret)))
	}

	// Mail settings only matter when something will actually try to send.
	if c.Auth.RequireEmailVerification {
		if c.Mail.FromAddress == "" {
			problems = append(problems, "MAIL_FROM_ADDRESS is required when AUTH_REQUIRE_EMAIL_VERIFICATION=true")
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

// envList reads a comma-separated variable into a slice, dropping blanks.
func envList(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
	}
	return fallback
}
