package correlation

import (
	"testing"
	"time"
)

func TestRetryStormDetector_BelowThreshold(t *testing.T) {
	d := NewRetryStormDetector(10*time.Second, 5)
	now := time.Now()

	for i := 0; i < 4; i++ {
		storm := d.Record("pod-a", now.Add(time.Duration(i)*time.Second))
		if storm {
			t.Errorf("should not be storm at count %d", i+1)
		}
	}

	if d.IsStorm("pod-a", now.Add(4*time.Second)) {
		t.Error("should not be storm with 4 events")
	}
}

func TestRetryStormDetector_ReachesThreshold(t *testing.T) {
	d := NewRetryStormDetector(10*time.Second, 5)
	now := time.Now()

	for i := 0; i < 4; i++ {
		d.Record("pod-a", now.Add(time.Duration(i)*time.Second))
	}

	storm := d.Record("pod-a", now.Add(4*time.Second))
	if !storm {
		t.Error("should be storm at count 5")
	}

	if !d.IsStorm("pod-a", now.Add(4*time.Second)) {
		t.Error("IsStorm should return true")
	}
}

func TestRetryStormDetector_WindowExpiry(t *testing.T) {
	d := NewRetryStormDetector(10*time.Second, 5)
	now := time.Now()

	// Add 5 events at time 0
	for i := 0; i < 5; i++ {
		d.Record("pod-a", now)
	}

	if !d.IsStorm("pod-a", now) {
		t.Error("should be storm immediately after 5 events")
	}

	// 11 seconds later, all events should have expired
	if d.IsStorm("pod-a", now.Add(11*time.Second)) {
		t.Error("should not be storm after window expiry")
	}

	if d.Count("pod-a", now.Add(11*time.Second)) != 0 {
		t.Error("count should be 0 after window expiry")
	}
}

func TestRetryStormDetector_MultiplePods(t *testing.T) {
	d := NewRetryStormDetector(10*time.Second, 3)
	now := time.Now()

	for i := 0; i < 3; i++ {
		d.Record("pod-a", now.Add(time.Duration(i)*time.Second))
	}
	d.Record("pod-b", now)

	if !d.IsStorm("pod-a", now.Add(2*time.Second)) {
		t.Error("pod-a should be in storm")
	}
	if d.IsStorm("pod-b", now.Add(2*time.Second)) {
		t.Error("pod-b should not be in storm")
	}
}

func TestRetryStormDetector_Reset(t *testing.T) {
	d := NewRetryStormDetector(10*time.Second, 3)
	now := time.Now()

	for i := 0; i < 5; i++ {
		d.Record("pod-a", now)
	}

	d.Reset()

	if d.IsStorm("pod-a", now) {
		t.Error("should not be storm after reset")
	}
	if d.Count("pod-a", now) != 0 {
		t.Error("count should be 0 after reset")
	}
}

func TestRetryStormDetector_UnknownPod(t *testing.T) {
	d := NewRetryStormDetector(10*time.Second, 5)
	now := time.Now()

	if d.IsStorm("nonexistent", now) {
		t.Error("unknown pod should not be in storm")
	}
	if d.Count("nonexistent", now) != 0 {
		t.Error("unknown pod count should be 0")
	}
}
