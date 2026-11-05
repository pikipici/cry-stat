package tradingconfig

import (
	"errors"
	"net/url"
	"strings"
)

const ProductModeMemeResearch = "meme_research"

type LookupEnv func(string) string

type Config struct {
	Enabled     bool
	DatabaseURL string
}

func Load(lookup LookupEnv) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("environment lookup is required")
	}
	if strings.ToLower(strings.TrimSpace(lookup("PRODUCT_MODE"))) != ProductModeMemeResearch {
		return Config{}, nil
	}

	databaseURL := strings.TrimSpace(lookup("TRADING_DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, errors.New("TRADING_DATABASE_URL is required in meme_research mode")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" || strings.Trim(parsed.Path, "/") == "" {
		return Config{}, errors.New("TRADING_DATABASE_URL is invalid")
	}
	return Config{Enabled: true, DatabaseURL: databaseURL}, nil
}
