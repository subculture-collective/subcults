package stream

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}

	// Verify all collectors are initialized (including new audio quality metrics)
	collectors := m.Collectors()
	if len(collectors) != 10 {
		t.Errorf("expected 10 collectors, got %d", len(collectors))
	}
}

func TestMetrics_Register(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		m := NewMetrics()
		reg := prometheus.NewRegistry()

		if err := m.Register(reg); err != nil {
			t.Errorf("Register() returned error: %v", err)
		}

		// Verify metrics are gathered
		families, err := reg.Gather()
		if err != nil {
			t.Errorf("Gather() returned error: %v", err)
		}

		expectedNames := map[string]bool{
			MetricStreamJoins:        false,
			MetricStreamLeaves:       false,
			MetricStreamJoinLatency:  false,
			MetricAudioBitrate:       false,
			MetricAudioJitter:        false,
			MetricAudioPacketLoss:    false,
			MetricAudioLevel:         false,
			MetricNetworkRTT:         false,
			MetricQualityAlerts:      false,
			MetricHighPacketLoss:     false,
		}

		for _, family := range families {
			if _, ok := expectedNames[family.GetName()]; ok {
				expectedNames[family.GetName()] = true
			}
		}

		for name, found := range expectedNames {
			if !found {
				t.Errorf("metric %s not found in gathered metrics", name)
			}
		}
	})

	t.Run("duplicate registration fails", func(t *testing.T) {
		m1 := NewMetrics()
		m2 := NewMetrics()
		reg := prometheus.NewRegistry()

		if err := m1.Register(reg); err != nil {
			t.Fatalf("first Register() returned error: %v", err)
		}

		if err := m2.Register(reg); err == nil {
			t.Error("second Register() should have returned an error")
		}
	})
}

func getCounterValue(c prometheus.Counter) float64 {
	var m dto.Metric
	if err := c.(prometheus.Metric).Write(&m); err != nil {
		return -1
	}
	return m.GetCounter().GetValue()
}

func TestMetrics_IncStreamJoins(t *testing.T) {
	m := NewMetrics()

	// Initial value should be 0
	initial := getCounterValue(m.streamJoins)
	if initial != 0 {
		t.Errorf("initial value = %f, want 0", initial)
	}

	// Increment 100 times
	for i := 0; i < 100; i++ {
		m.IncStreamJoins()
	}

	final := getCounterValue(m.streamJoins)
	if final != 100 {
		t.Errorf("final value = %f, want 100", final)
	}
}

func TestMetrics_IncStreamLeaves(t *testing.T) {
	m := NewMetrics()

	initial := getCounterValue(m.streamLeaves)
	if initial != 0 {
		t.Errorf("initial value = %f, want 0", initial)
	}

	for i := 0; i < 50; i++ {
		m.IncStreamLeaves()
	}

	final := getCounterValue(m.streamLeaves)
	if final != 50 {
		t.Errorf("final value = %f, want 50", final)
	}
}

func getHistogramSampleCount(h prometheus.Histogram) uint64 {
	var m dto.Metric
	if err := h.(prometheus.Metric).Write(&m); err != nil {
		return 0
	}
	return m.GetHistogram().GetSampleCount()
}

func getHistogramSampleSum(h prometheus.Histogram) float64 {
	var m dto.Metric
	if err := h.(prometheus.Metric).Write(&m); err != nil {
		return -1
	}
	return m.GetHistogram().GetSampleSum()
}

func TestMetrics_ObserveStreamJoinLatency(t *testing.T) {
	m := NewMetrics()

	// Initial count should be 0
	initial := getHistogramSampleCount(m.streamJoinLatency)
	if initial != 0 {
		t.Errorf("initial sample count = %d, want 0", initial)
	}

	// Observe some latencies (simulating real-world join times)
	latencies := []float64{0.5, 1.2, 0.8, 2.5, 1.0, 3.2, 0.9, 1.5}
	var expectedSum float64
	for _, l := range latencies {
		m.ObserveStreamJoinLatency(l)
		expectedSum += l
	}

	finalCount := getHistogramSampleCount(m.streamJoinLatency)
	if finalCount != uint64(len(latencies)) {
		t.Errorf("final sample count = %d, want %d", finalCount, len(latencies))
	}

	finalSum := getHistogramSampleSum(m.streamJoinLatency)
	// Use approximate comparison for floating point
	if finalSum < expectedSum*0.99 || finalSum > expectedSum*1.01 {
		t.Errorf("final sample sum = %f, want approximately %f", finalSum, expectedSum)
	}
}

func TestMetrics_Concurrency(t *testing.T) {
	m := NewMetrics()
	done := make(chan bool)
	iterations := 100

	// Run concurrent operations on all metrics
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				m.IncStreamJoins()
				m.IncStreamLeaves()
				m.ObserveStreamJoinLatency(1.5)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	expected := float64(10 * iterations)

	if v := getCounterValue(m.streamJoins); v != expected {
		t.Errorf("streamJoins = %f, want %f", v, expected)
	}
	if v := getCounterValue(m.streamLeaves); v != expected {
		t.Errorf("streamLeaves = %f, want %f", v, expected)
	}

	expectedHistCount := uint64(10 * iterations)
	if c := getHistogramSampleCount(m.streamJoinLatency); c != expectedHistCount {
		t.Errorf("streamJoinLatency sample count = %d, want %d", c, expectedHistCount)
	}
}
