package main

import (
	"math"
	"testing"
)

// --- WindowState Tests ---

func TestWindowState_CountsCorrectly(t *testing.T) {
	w := &WindowState{}
	for i := 0; i < 100; i++ {
		w.Add(float64(i))
	}
	if w.Count != 100 {
		t.Errorf("expected Count=100, got %d", w.Count)
	}
}

func TestWindowState_Avg(t *testing.T) {
	w := &WindowState{}
	w.Add(10)
	w.Add(20)
	w.Add(30)
	avg := w.Avg()
	if avg != 20.0 {
		t.Errorf("expected avg=20.0, got %.2f", avg)
	}
}

func TestWindowState_Avg_Empty(t *testing.T) {
	w := &WindowState{}
	if w.Avg() != 0 {
		t.Error("expected 0 for empty window")
	}
}

func TestWindowState_P99(t *testing.T) {
	w := &WindowState{}
	for i := 1; i <= 100; i++ {
		w.Add(float64(i))
	}
	p99 := w.P99()
	if p99 < 98 || p99 > 100 {
		t.Errorf("expected p99 near 99, got %.2f", p99)
	}
}

func TestWindowState_P99_Empty(t *testing.T) {
	w := &WindowState{}
	if w.P99() != 0 {
		t.Error("expected 0 for empty window")
	}
}

func TestWindowState_ConcurrentAccess(t *testing.T) {
	w := &WindowState{}
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(v float64) {
			for j := 0; j < 100; j++ {
				w.Add(v)
			}
			done <- struct{}{}
		}(float64(i))
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if w.Count != 1000 {
		t.Errorf("expected Count=1000, got %d", w.Count)
	}
}

// --- AnomalyDetector Tests ---

func TestAnomalyDetector_NotEnoughHistory(t *testing.T) {
	d := newTestDetector()
	for i := 0; i < 9; i++ {
		isAnomaly, _ := d.Check("acme", "purchase", 100.0)
		if isAnomaly {
			t.Errorf("iteration %d: should not flag anomaly with < 10 samples", i)
		}
	}
}

func TestAnomalyDetector_NormalTraffic(t *testing.T) {
	d := newTestDetector()
	for i := 0; i < 20; i++ {
		d.Check("acme", "click", 100.0)
	}
	isAnomaly, zscore := d.Check("acme", "click", 101.0)
	if isAnomaly {
		t.Errorf("normal traffic should not be anomalous, z=%.2f", zscore)
	}
}

func TestAnomalyDetector_SpikeDetected(t *testing.T) {
	d := newTestDetector()
	// Use slightly varied history so stddev > 0
	base := []float64{98, 102, 99, 101, 100, 103, 97, 101, 99, 100, 98, 102, 101, 99, 100, 103, 97, 98, 102, 101}
	for _, v := range base {
		d.Check("acme", "purchase", v)
	}
	isAnomaly, zscore := d.Check("acme", "purchase", 1000.0)
	if !isAnomaly {
		t.Errorf("10x spike should be anomalous, z=%.2f", zscore)
	}
	if zscore <= 3.0 {
		t.Errorf("expected z>3.0, got %.2f", zscore)
	}
}

func TestAnomalyDetector_DropDetected(t *testing.T) {
	d := newTestDetector()
	// Use slightly varied history so stddev > 0
	base := []float64{498, 502, 499, 501, 500, 503, 497, 501, 499, 500, 498, 502, 501, 499, 500, 503, 497, 498, 502, 501}
	for _, v := range base {
		d.Check("acme", "login", v)
	}
	isAnomaly, _ := d.Check("acme", "login", 0.0)
	if !isAnomaly {
		t.Error("drop to zero should be anomalous")
	}
}

func TestAnomalyDetector_MultiTenantIsolated(t *testing.T) {
	d := newTestDetector()
	for i := 0; i < 20; i++ {
		d.Check("tenant-a", "click", 1000.0)
	}
	// tenant-b has no history — should not inherit tenant-a's
	isAnomaly, _ := d.Check("tenant-b", "click", 1000.0)
	if isAnomaly {
		t.Error("tenant-b should not inherit tenant-a history")
	}
}

func TestAnomalyDetector_ZeroStddevNoPanic(t *testing.T) {
	d := newTestDetector()
	// Perfectly constant — stddev=0, must not divide by zero
	for i := 0; i < 20; i++ {
		d.Check("acme", "ping", 50.0)
	}
	// Should not panic
	d.Check("acme", "ping", 51.0)
}

// --- Helpers ---

func newTestDetector() *AnomalyDetector {
	return &AnomalyDetector{
		history: make(map[string][]float64),
	}
}

// Satisfy math import
var _ = math.Sqrt
// window count
// avg p99
// empty
// anomaly
// concurrent
// isolation
// window count
// avg p99
// empty
// anomaly
// concurrent
// isolation
// window count
// avg p99
// empty
// anomaly
// concurrent
// isolation
// count test
// avg test
// avg empty
// p99 test
// p99 empty
// concurrent
// no history
// normal traffic
// spike
// drop
// isolation
// zero stddev
// varied history
// count test
// avg test
// avg empty
// p99 test
// p99 empty
// concurrent
// no history
// normal traffic
// spike
// drop
// isolation
// zero stddev
// varied history
// count test
// avg test
// avg empty
// p99 test
// p99 empty
// concurrent
// no history
// normal traffic
