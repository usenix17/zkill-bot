package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from the environment.
type Config struct {
	// Rules
	RulesFilePath string

	// EVE SDE enrichment
	EVEDBPath string

	// State persistence
	StateFilePath string

	// R2Z2 API
	R2Z2BaseURL      string
	R2Z2SequencePath string

	// Polling behaviour
	PollInterval   time.Duration
	Poll404Backoff time.Duration

	// Retry behaviour for actions
	RetryMaxRetries   int
	RetryBaseBackoff  time.Duration
	RetryMaxBackoff   time.Duration

	// Observability
	Debug                   bool
	MetricsLogInterval      time.Duration
	ObsAlertWebhookURL      string
	ObsRepeated403Threshold int
	ObsRepeated429Threshold int
	ObsStalledSequenceMS    time.Duration
}

// Load reads .env (if present) and then os.Getenv, validates all values, and
// returns a fully-populated Config or an error with actionable detail.
func Load() (*Config, error) {
	// Load .env file if it exists; ignore "file not found" errors.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: loading .env: %w", err)
	}

	var errs []string
	add := func(msg string) { errs = append(errs, "  - "+msg) }

	c := &Config{}

	// --- Rules ---
	c.RulesFilePath = envStr("RULES_FILE_PATH", "./config/rules.yaml")

	// --- EVE DB ---
	c.EVEDBPath = envStr("EVE_DB_PATH", "./eve.db")
	if _, err := os.Stat(c.EVEDBPath); err != nil {
		add(fmt.Sprintf("EVE_DB_PATH=%q: file not found", c.EVEDBPath))
	}

	// --- State file ---
	c.StateFilePath = envStr("STATE_FILE_PATH", "./state.json")

	// --- R2Z2 ---
	c.R2Z2BaseURL = envStr("R2Z2_BASE_URL", "https://r2z2.zkillboard.com")
	if _, err := url.ParseRequestURI(c.R2Z2BaseURL); err != nil {
		add(fmt.Sprintf("R2Z2_BASE_URL=%q: invalid URL: %v", c.R2Z2BaseURL, err))
	}
	c.R2Z2SequencePath = envStr("R2Z2_SEQUENCE_PATH", "/ephemeral/sequence.json")

	// --- Polling ---
	c.PollInterval = envDuration("POLL_INTERVAL_MS", 100*time.Millisecond, add)
	c.Poll404Backoff = envDuration("POLL_404_BACKOFF_MS", 6*time.Second, add)

	// --- Retries ---
	c.RetryMaxRetries = envInt("RETRY_MAX_RETRIES", 3, add)
	c.RetryBaseBackoff = envDuration("RETRY_BASE_BACKOFF_MS", 250*time.Millisecond, add)
	c.RetryMaxBackoff = envDuration("RETRY_MAX_BACKOFF_MS", 10*time.Second, add)

	// --- Observability ---
	c.Debug = strings.ToLower(envStr("DEBUG", "false")) == "true"
	c.MetricsLogInterval = envDuration("OBS_METRICS_LOG_INTERVAL_MS", 60*time.Second, add)
	c.ObsAlertWebhookURL = envStr("OBS_ALERT_WEBHOOK_URL", "")
	if c.ObsAlertWebhookURL != "" {
		if _, err := url.ParseRequestURI(c.ObsAlertWebhookURL); err != nil {
			add(fmt.Sprintf("OBS_ALERT_WEBHOOK_URL=%q: invalid URL: %v", c.ObsAlertWebhookURL, err))
		}
	}
	c.ObsRepeated403Threshold = envInt("OBS_REPEATED_403_THRESHOLD", 3, add)
	c.ObsRepeated429Threshold = envInt("OBS_REPEATED_429_THRESHOLD", 5, add)
	c.ObsStalledSequenceMS = envDuration("OBS_STALLED_SEQUENCE_MS", 15*time.Minute, add)

	if len(errs) > 0 {
		return nil, errors.New("config: validation failed:\n" + strings.Join(errs, "\n"))
	}

	return c, nil
}

// --- helpers ---

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int, add func(string)) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		add(fmt.Sprintf("%s=%q: must be an integer", key, v))
		return def
	}
	return n
}

// envDuration reads a millisecond integer env var and returns a time.Duration.
func envDuration(key string, def time.Duration, add func(string)) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		add(fmt.Sprintf("%s=%q: must be a non-negative integer (milliseconds)", key, v))
		return def
	}
	return time.Duration(n) * time.Millisecond
}
