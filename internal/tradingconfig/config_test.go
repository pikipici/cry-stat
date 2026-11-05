package tradingconfig

import (
	"strings"
	"testing"
)

func TestLoadNormalizesMemeResearchMode(t *testing.T) {
	_, err := Load(func(key string) string {
		if key == "PRODUCT_MODE" {
			return "  MEME_RESEARCH  "
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "TRADING_DATABASE_URL") {
		t.Fatalf("error = %v, mau normalized meme_research require DSN", err)
	}
}

func TestLoadRequiresTradingDatabaseURLInMemeResearch(t *testing.T) {
	_, err := Load(func(key string) string {
		if key == "PRODUCT_MODE" {
			return "meme_research"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "TRADING_DATABASE_URL") {
		t.Fatalf("error = %v, mau TRADING_DATABASE_URL required", err)
	}
}

func TestLoadDoesNotRequireTradingDatabaseURLInLegacySandbox(t *testing.T) {
	config, err := Load(func(key string) string {
		if key == "PRODUCT_MODE" {
			return "legacy_sandbox"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.Enabled || config.DatabaseURL != "" {
		t.Fatalf("legacy config = %+v, mau disabled tanpa DSN", config)
	}
}

func TestLoadMemeResearchKeepsTradingDatabaseURL(t *testing.T) {
	const dsn = "postgres://research:secret@localhost/trading?sslmode=disable"
	config, err := Load(func(key string) string {
		switch key {
		case "PRODUCT_MODE":
			return "meme_research"
		case "TRADING_DATABASE_URL":
			return dsn
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !config.Enabled || config.DatabaseURL != dsn {
		t.Fatalf("meme research config = %+v", config)
	}
}

func TestLoadErrorsNeverLeakTradingDatabaseURL(t *testing.T) {
	const dsn = "not-a-postgres-url-with-secret"
	_, err := Load(func(key string) string {
		switch key {
		case "PRODUCT_MODE":
			return "meme_research"
		case "TRADING_DATABASE_URL":
			return dsn
		default:
			return ""
		}
	})
	if err == nil {
		t.Fatal("Load() error = nil, mau invalid URL")
	}
	if strings.Contains(err.Error(), dsn) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error membocorkan DSN: %v", err)
	}
}
