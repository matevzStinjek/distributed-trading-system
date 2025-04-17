package config

type Config struct {
	Symbols          []string
	TradeChannelBuff int
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Symbols:          []string{"AAPL", "MSFT", "GOOG", "AMZN", "TSLA"},
		TradeChannelBuff: 100,
	}
	return cfg, nil
}
