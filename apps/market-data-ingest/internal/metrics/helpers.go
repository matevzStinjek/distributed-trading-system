package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Timer provides a convenient way to time operations
type Timer struct {
	histogram prometheus.Observer
	startTime time.Time
}

// NewTimer creates a new timer using the given histogram
func NewTimer(histogram prometheus.Observer) *Timer {
	return &Timer{
		histogram: histogram,
		startTime: time.Now(),
	}
}

// ObserveDuration records the duration since the timer was created
func (t *Timer) ObserveDuration() {
	t.histogram.Observe(time.Since(t.startTime).Seconds())
}

// TimeFunc executes the given function and records its execution time
// in the provided histogram
func TimeFunc(histogram prometheus.Observer, f func()) {
	timer := NewTimer(histogram)
	defer timer.ObserveDuration()
	f()
}

// TimeFuncWithResult executes the given function, records its execution time,
// and returns the function's result
func TimeFuncWithResult[T any](histogram prometheus.Observer, f func() T) T {
	timer := NewTimer(histogram)
	defer timer.ObserveDuration()
	return f()
}
