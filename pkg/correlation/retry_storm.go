package correlation

import (
	"sync"
	"time"
)

const (
	// DefaultStormWindow is the time window for counting retransmit events.
	DefaultStormWindow = 10 * time.Second

	// DefaultStormThreshold is the minimum retransmit count in a window
	// to classify as a retry storm.
	DefaultStormThreshold = 5
)

// RetryStormDetector identifies bursts of TCP retransmissions within a
// sliding window, per pod. When the count exceeds the threshold, the
// detector flags the pod as experiencing a retry storm and emits
// llm.ebpf.tcp.retry_storm=true on correlated spans.
type RetryStormDetector struct {
	mu        sync.Mutex
	window    time.Duration
	threshold int
	buckets   map[string]*stormBucket // keyed by pod name
}

type stormBucket struct {
	events []time.Time
}

// NewRetryStormDetector creates a detector with the given window and
// threshold. Use DefaultStormWindow and DefaultStormThreshold for
// recommended defaults.
func NewRetryStormDetector(window time.Duration, threshold int) *RetryStormDetector {
	return &RetryStormDetector{
		window:    window,
		threshold: threshold,
		buckets:   make(map[string]*stormBucket),
	}
}

// Record registers a TCP retransmit event for the given pod at the
// specified timestamp. Returns true if this event pushes the pod
// into storm state.
func (d *RetryStormDetector) Record(pod string, ts time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	bucket, ok := d.buckets[pod]
	if !ok {
		bucket = &stormBucket{}
		d.buckets[pod] = bucket
	}

	bucket.events = append(bucket.events, ts)
	bucket.events = pruneOld(bucket.events, ts, d.window)

	return len(bucket.events) >= d.threshold
}

// IsStorm checks whether the given pod is currently in a retry storm
// state (count >= threshold within the window ending at now).
func (d *RetryStormDetector) IsStorm(pod string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	bucket, ok := d.buckets[pod]
	if !ok {
		return false
	}

	bucket.events = pruneOld(bucket.events, now, d.window)
	return len(bucket.events) >= d.threshold
}

// Count returns the current retransmit count within the window for a pod.
func (d *RetryStormDetector) Count(pod string, now time.Time) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	bucket, ok := d.buckets[pod]
	if !ok {
		return 0
	}

	bucket.events = pruneOld(bucket.events, now, d.window)
	return len(bucket.events)
}

// Reset clears all tracked state.
func (d *RetryStormDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.buckets = make(map[string]*stormBucket)
}

func pruneOld(events []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	start := 0
	for start < len(events) && events[start].Before(cutoff) {
		start++
	}
	if start == 0 {
		return events
	}
	pruned := make([]time.Time, len(events)-start)
	copy(pruned, events[start:])
	return pruned
}
