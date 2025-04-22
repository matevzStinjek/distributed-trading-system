package aggregator

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type TradeAggregator struct {
	mutex        sync.Mutex
	latestTrades map[string]marketdata.Trade
	cfg          *config.Config
}

func (ta *TradeAggregator) Start(
	ctx context.Context,
	rawTradesChan <-chan marketdata.Trade,
	processedTradesChan chan<- marketdata.Trade,
	wg *sync.WaitGroup,
) error {
	defer func() {
		close(processedTradesChan)
		wg.Done()
	}()

	ticker := time.NewTicker(ta.cfg.AggregatorInterval)
	defer ticker.Stop()

	for {
		select {
		case trade, ok := <-rawTradesChan:
			if !ok {
				ta.flushPrices(processedTradesChan)
				return nil
			}
			// update trade
			ta.mutex.Lock()
			ta.latestTrades[trade.Symbol] = trade
			ta.mutex.Unlock()

		case <-ticker.C:
			ta.flushPrices(processedTradesChan)
		case <-ctx.Done():
			ta.flushPrices(processedTradesChan)
			return ctx.Err()
		}
	}
}

func (ta *TradeAggregator) flushPrices(processedTradesChan chan<- marketdata.Trade) {
	if len(ta.latestTrades) == 0 {
		return
	}

	ta.mutex.Lock()
	tradesToSend := make([]marketdata.Trade, 0, len(ta.latestTrades))
	for _, trade := range ta.latestTrades {
		tradesToSend = append(tradesToSend, trade)
	}
	ta.latestTrades = make(map[string]marketdata.Trade)
	ta.mutex.Unlock()

	for _, trade := range tradesToSend {
		select {
		case processedTradesChan <- trade:
		default:
			log.Printf("WARN: processedTradesChan full, dropping trade")
		}
	}
}

func NewTradeAggregator(cfg *config.Config) *TradeAggregator {
	return &TradeAggregator{
		latestTrades: make(map[string]marketdata.Trade),
		cfg:          cfg,
	}
}
