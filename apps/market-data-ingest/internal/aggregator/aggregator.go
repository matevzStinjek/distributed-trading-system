package aggregator

import (
	"context"
	"sync"
	"time"

	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/internal/config"
	"github.com/matevzStinjek/distributed-trading-system/market-data-ingest/pkg/marketdata"
)

type TradeAggregator struct {
	mutex        sync.Mutex
	latestPrices map[string]marketdata.Trade
	cfg          config.Config
}

func (ta *TradeAggregator) Start(
	ctx context.Context,
	rawTradesChan <-chan marketdata.Trade,
	logChan chan marketdata.Trade,
) {
	ticker := time.NewTicker(ta.cfg.AggregatorInterval)
	defer ticker.Stop()

	for {
		select {
		case trade, ok := <-rawTradesChan:
			if !ok {
				ta.flushPrices()
				return
			}
			// update price
			ta.mutex.Lock()
			ta.latestPrices[trade.Symbol] = trade
			ta.mutex.Unlock()

			// relay trade to log
			logChan <- trade

		case <-ticker.C:
			ta.flushPrices()
		case <-ctx.Done():
			ta.flushPrices()
			return
		}
	}
}

func (ta *TradeAggregator) flushPrices() {

}
