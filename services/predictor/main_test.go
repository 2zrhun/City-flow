package main

import (
	"math"
	"os"
	"testing"
)

func TestComputeCongestionScore(t *testing.T) {
	tests := []struct {
		name        string
		avgSpeed    float64
		avgOcc      float64
		avgFlow     float64
		wantMin     float64
		wantMax     float64
	}{
		{
			name:     "free flow: high speed, low occupancy, low flow",
			avgSpeed: 80.0, avgOcc: 0.1, avgFlow: 20.0,
			wantMin: 0.0, wantMax: 0.15,
		},
		{
			name:     "heavy congestion: low speed, high occupancy, high flow",
			avgSpeed: 10.0, avgOcc: 0.9, avgFlow: 100.0,
			wantMin: 0.85, wantMax: 1.0,
		},
		{
			name:     "moderate: medium speed, medium occupancy",
			avgSpeed: 45.0, avgOcc: 0.5, avgFlow: 60.0,
			wantMin: 0.3, wantMax: 0.6,
		},
		{
			name:     "zero values",
			avgSpeed: 0.0, avgOcc: 0.0, avgFlow: 0.0,
			wantMin: 0.4, wantMax: 0.4,
		},
		{
			name:     "max speed returns near zero",
			avgSpeed: maxSpeed, avgOcc: 0.0, avgFlow: 0.0,
			wantMin: 0.0, wantMax: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCongestionScore(tt.avgSpeed, tt.avgOcc, tt.avgFlow)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeCongestionScore(%v, %v, %v) = %v, want [%v, %v]",
					tt.avgSpeed, tt.avgOcc, tt.avgFlow, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestComputeCongestionScoreClamped(t *testing.T) {
	t.Run("never exceeds 1.0", func(t *testing.T) {
		score := computeCongestionScore(0, 1.5, 200.0)
		if score > 1.0 {
			t.Errorf("score = %v, should be clamped to 1.0", score)
		}
	})

	t.Run("never below 0.0", func(t *testing.T) {
		score := computeCongestionScore(200, 0, 0)
		if score < 0.0 {
			t.Errorf("score = %v, should be clamped to 0.0", score)
		}
	})
}

func TestComputeCongestionScoreWeights(t *testing.T) {
	// Speed weight = 0.4, Occupancy weight = 0.4, Flow weight = 0.2
	// With avgSpeed=45 (speedScore=0.5), avgOcc=0.5, avgFlow=60 (flowScore=0.5):
	// expected = 0.4*0.5 + 0.4*0.5 + 0.2*0.5 = 0.5
	score := computeCongestionScore(45.0, 0.5, 60.0)
	if math.Abs(score-0.5) > 0.001 {
		t.Errorf("expected ~0.5 for balanced inputs, got %v", score)
	}
}

func TestGetEnv(t *testing.T) {
	os.Unsetenv("TEST_PREDICTOR_VAR")
	if got := getEnv("TEST_PREDICTOR_VAR", "fallback"); got != "fallback" {
		t.Errorf("getEnv() = %q, want %q", got, "fallback")
	}

	os.Setenv("TEST_PREDICTOR_VAR", "custom")
	defer os.Unsetenv("TEST_PREDICTOR_VAR")
	if got := getEnv("TEST_PREDICTOR_VAR", "fallback"); got != "custom" {
		t.Errorf("getEnv() = %q, want %q", got, "custom")
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Run("returns fallback when unset", func(t *testing.T) {
		os.Unsetenv("TEST_INT_VAR")
		if got := getEnvInt("TEST_INT_VAR", 42); got != 42 {
			t.Errorf("getEnvInt() = %d, want %d", got, 42)
		}
	})

	t.Run("parses valid integer", func(t *testing.T) {
		os.Setenv("TEST_INT_VAR", "100")
		defer os.Unsetenv("TEST_INT_VAR")
		if got := getEnvInt("TEST_INT_VAR", 42); got != 100 {
			t.Errorf("getEnvInt() = %d, want %d", got, 100)
		}
	})

	t.Run("returns fallback for invalid value", func(t *testing.T) {
		os.Setenv("TEST_INT_VAR", "not_a_number")
		defer os.Unsetenv("TEST_INT_VAR")
		if got := getEnvInt("TEST_INT_VAR", 42); got != 42 {
			t.Errorf("getEnvInt() = %d, want %d", got, 42)
		}
	})
}

func TestPredictionStruct(t *testing.T) {
	p := Prediction{
		RoadID:          "RING-NORTH-12",
		HorizonMin:      30,
		CongestionScore: 0.75,
		Confidence:      0.9,
		ModelVersion:    "baseline-v1",
	}
	if p.RoadID != "RING-NORTH-12" {
		t.Errorf("RoadID = %q", p.RoadID)
	}
	if p.CongestionScore != 0.75 {
		t.Errorf("CongestionScore = %v", p.CongestionScore)
	}
}
