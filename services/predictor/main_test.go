package main

import (
	"math"
	"os"
	"testing"
)

// ── computeCongestionScore tests (unchanged) ──

func TestComputeCongestionScore(t *testing.T) {
	tests := []struct {
		name    string
		speed   float64
		occ     float64
		flow    float64
		wantMin float64
		wantMax float64
	}{
		{"free flow", 80.0, 0.1, 20.0, 0.0, 0.15},
		{"heavy congestion", 10.0, 0.9, 100.0, 0.85, 1.0},
		{"moderate", 45.0, 0.5, 60.0, 0.3, 0.6},
		{"zero values", 0.0, 0.0, 0.0, 0.4, 0.4},
		{"max speed", maxSpeed, 0.0, 0.0, 0.0, 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCongestionScore(tt.speed, tt.occ, tt.flow)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("got %v, want [%v, %v]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestComputeCongestionScoreClamped(t *testing.T) {
	if score := computeCongestionScore(0, 1.5, 200.0); score > 1.0 {
		t.Errorf("score = %v, should be clamped to 1.0", score)
	}
	if score := computeCongestionScore(200, 0, 0); score < 0.0 {
		t.Errorf("score = %v, should be clamped to 0.0", score)
	}
}

func TestComputeCongestionScoreWeights(t *testing.T) {
	score := computeCongestionScore(45.0, 0.5, 60.0)
	if math.Abs(score-0.5) > 0.001 {
		t.Errorf("expected ~0.5, got %v", score)
	}
}

// ── EWMA tests ──

func TestEWMA(t *testing.T) {
	tests := []struct {
		name      string
		predicted float64
		current   float64
		alpha     float64
		want      float64
	}{
		{"alpha=1.0 returns predicted", 0.8, 0.3, 1.0, 0.8},
		{"alpha=0.0 returns current", 0.8, 0.3, 0.0, 0.3},
		{"alpha=0.5 returns midpoint", 0.8, 0.2, 0.5, 0.5},
		{"default alpha=0.7", 1.0, 0.0, 0.7, 0.7},
		{"equal values unchanged", 0.5, 0.5, 0.7, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ewma(tt.predicted, tt.current, tt.alpha)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("ewma(%v, %v, %v) = %v, want %v", tt.predicted, tt.current, tt.alpha, got, tt.want)
			}
		})
	}
}

// ── Rush hour factor tests ──

func TestRushHourFactor(t *testing.T) {
	tests := []struct {
		hour int
		want float64
	}{
		{7, 1.15},  // morning rush
		{8, 1.15},
		{17, 1.15}, // evening rush
		{18, 1.15},
		{9, 1.0},   // normal daytime
		{12, 1.0},
		{16, 1.0},
		{21, 0.85},  // night
		{0, 0.85},
		{3, 0.85},
		{5, 0.85},
		{6, 1.0},   // early morning (not night)
		{19, 1.0},  // after evening rush
		{20, 1.0},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := rushHourFactor(tt.hour)
			if got != tt.want {
				t.Errorf("rushHourFactor(%d) = %v, want %v", tt.hour, got, tt.want)
			}
		})
	}
}

// ── Linear regression tests ──

func TestFitLinearRegression(t *testing.T) {
	t.Run("perfect positive trend", func(t *testing.T) {
		xs := []float64{0, 5, 10, 15, 20, 25}
		ys := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}
		slope, intercept := fitLinearRegression(xs, ys)
		if math.Abs(slope-0.02) > 0.001 {
			t.Errorf("slope = %v, want ~0.02", slope)
		}
		if math.Abs(intercept-0.1) > 0.001 {
			t.Errorf("intercept = %v, want ~0.1", intercept)
		}
	})

	t.Run("flat trend", func(t *testing.T) {
		xs := []float64{0, 5, 10, 15, 20, 25}
		ys := []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5}
		slope, intercept := fitLinearRegression(xs, ys)
		if math.Abs(slope) > 0.001 {
			t.Errorf("slope = %v, want ~0", slope)
		}
		if math.Abs(intercept-0.5) > 0.001 {
			t.Errorf("intercept = %v, want ~0.5", intercept)
		}
	})

	t.Run("negative trend (improving congestion)", func(t *testing.T) {
		xs := []float64{0, 5, 10, 15, 20, 25}
		ys := []float64{0.9, 0.8, 0.7, 0.6, 0.5, 0.4}
		slope, _ := fitLinearRegression(xs, ys)
		if slope >= 0 {
			t.Errorf("slope = %v, should be negative for decreasing congestion", slope)
		}
	})

	t.Run("single point fallback", func(t *testing.T) {
		xs := []float64{5.0}
		ys := []float64{0.6}
		slope, intercept := fitLinearRegression(xs, ys)
		if slope != 0 {
			t.Errorf("slope = %v, want 0 for single point", slope)
		}
		if intercept != 0.6 {
			t.Errorf("intercept = %v, want 0.6 for single point", intercept)
		}
	})

	t.Run("extrapolation gives expected value", func(t *testing.T) {
		// y = 0.02*x + 0.1 → at x=60 (30min lookback + 30min horizon): y = 1.3
		xs := []float64{0, 5, 10, 15, 20, 25}
		ys := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}
		slope, intercept := fitLinearRegression(xs, ys)
		predicted := slope*60 + intercept
		if math.Abs(predicted-1.3) > 0.01 {
			t.Errorf("predicted at t=60 = %v, want ~1.3", predicted)
		}
	})
}

// ── End-to-end prediction pipeline test ──

func TestPredictionPipeline(t *testing.T) {
	t.Run("increasing congestion predicts higher score", func(t *testing.T) {
		// Simulate 6 buckets of increasing congestion
		xs := []float64{0, 5, 10, 15, 20, 25}
		ys := []float64{0.2, 0.3, 0.35, 0.4, 0.45, 0.5}
		currentScore := ys[len(ys)-1]

		slope, intercept := fitLinearRegression(xs, ys)
		predicted := slope*60 + intercept // 30 min lookback + 30 min horizon
		blended := ewma(predicted, currentScore, ewmaAlpha)
		final := blended * rushHourFactor(12) // noon — factor = 1.0
		final = math.Max(0.0, math.Min(1.0, final))

		if final <= currentScore {
			t.Errorf("predicted score %v should be > current %v for increasing trend", final, currentScore)
		}
	})

	t.Run("decreasing congestion predicts lower score", func(t *testing.T) {
		xs := []float64{0, 5, 10, 15, 20, 25}
		ys := []float64{0.8, 0.7, 0.65, 0.6, 0.55, 0.5}
		currentScore := ys[len(ys)-1]

		slope, intercept := fitLinearRegression(xs, ys)
		predicted := slope*60 + intercept
		blended := ewma(predicted, currentScore, ewmaAlpha)
		final := blended * rushHourFactor(12)
		final = math.Max(0.0, math.Min(1.0, final))

		if final >= currentScore {
			t.Errorf("predicted score %v should be < current %v for decreasing trend", final, currentScore)
		}
	})

	t.Run("rush hour boosts prediction", func(t *testing.T) {
		xs := []float64{0, 5, 10, 15, 20, 25}
		ys := []float64{0.4, 0.4, 0.4, 0.4, 0.4, 0.4}

		slope, intercept := fitLinearRegression(xs, ys)
		predicted := slope*60 + intercept
		blended := ewma(predicted, 0.4, ewmaAlpha)

		normalHour := blended * rushHourFactor(12)
		rushHour := blended * rushHourFactor(8)

		if rushHour <= normalHour {
			t.Errorf("rush hour score %v should be > normal %v", rushHour, normalHour)
		}
	})

	t.Run("prediction always clamped 0-1", func(t *testing.T) {
		// Very steep upward trend → extrapolation > 1.0
		xs := []float64{0, 5, 10, 15, 20, 25}
		ys := []float64{0.5, 0.6, 0.7, 0.8, 0.9, 1.0}

		slope, intercept := fitLinearRegression(xs, ys)
		predicted := slope*60 + intercept
		blended := ewma(predicted, 1.0, ewmaAlpha)
		final := blended * rushHourFactor(8) // rush hour boost
		final = math.Max(0.0, math.Min(1.0, final))

		if final > 1.0 || final < 0.0 {
			t.Errorf("final score %v should be in [0, 1]", final)
		}
	})
}

// ── Env helper tests ──

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
	os.Unsetenv("TEST_INT_VAR")
	if got := getEnvInt("TEST_INT_VAR", 42); got != 42 {
		t.Errorf("getEnvInt() = %d, want %d", got, 42)
	}
	os.Setenv("TEST_INT_VAR", "100")
	defer os.Unsetenv("TEST_INT_VAR")
	if got := getEnvInt("TEST_INT_VAR", 42); got != 100 {
		t.Errorf("getEnvInt() = %d, want %d", got, 100)
	}
}
