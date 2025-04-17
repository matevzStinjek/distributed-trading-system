package marketdata

import (
	"time"
)

type Trade struct {
	ID        int64
	Symbol    string
	Price     float64
	Size      uint32
	Timestamp time.Time
}
