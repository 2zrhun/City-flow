package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	t.Run("returns fallback when unset", func(t *testing.T) {
		os.Unsetenv("TEST_COLLECTOR_VAR")
		got := getEnv("TEST_COLLECTOR_VAR", "default_val")
		if got != "default_val" {
			t.Errorf("getEnv() = %q, want %q", got, "default_val")
		}
	})

	t.Run("returns env value when set", func(t *testing.T) {
		os.Setenv("TEST_COLLECTOR_VAR", "custom")
		defer os.Unsetenv("TEST_COLLECTOR_VAR")
		got := getEnv("TEST_COLLECTOR_VAR", "default_val")
		if got != "custom" {
			t.Errorf("getEnv() = %q, want %q", got, "custom")
		}
	})
}

func TestTrafficPayloadJSON(t *testing.T) {
	t.Run("valid payload unmarshals correctly", func(t *testing.T) {
		raw := `{"ts":"2025-01-15T10:30:00Z","sensor_id":"PARIS-1643","road_id":"RING-NORTH-12","speed_kmh":42.5,"flow_rate":120.3,"occupancy":0.45}`
		var p TrafficPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if p.SensorID != "PARIS-1643" {
			t.Errorf("SensorID = %q, want %q", p.SensorID, "PARIS-1643")
		}
		if p.RoadID != "RING-NORTH-12" {
			t.Errorf("RoadID = %q, want %q", p.RoadID, "RING-NORTH-12")
		}
		if p.SpeedKMH != 42.5 {
			t.Errorf("SpeedKMH = %f, want %f", p.SpeedKMH, 42.5)
		}
		if p.FlowRate != 120.3 {
			t.Errorf("FlowRate = %f, want %f", p.FlowRate, 120.3)
		}
		if p.Occupancy != 0.45 {
			t.Errorf("Occupancy = %f, want %f", p.Occupancy, 0.45)
		}
	})

	t.Run("empty sensor_id detected", func(t *testing.T) {
		raw := `{"ts":"2025-01-15T10:30:00Z","sensor_id":"","road_id":"RING-NORTH-12","speed_kmh":42.5}`
		var p TrafficPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if p.SensorID != "" {
			t.Errorf("SensorID should be empty, got %q", p.SensorID)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		raw := `{not valid json}`
		var p TrafficPayload
		if err := json.Unmarshal([]byte(raw), &p); err == nil {
			t.Error("expected Unmarshal error for invalid JSON")
		}
	})

	t.Run("timestamp parses as RFC3339", func(t *testing.T) {
		raw := `{"ts":"2025-06-15T14:30:00Z","sensor_id":"S1","road_id":"R1","speed_kmh":50}`
		var p TrafficPayload
		json.Unmarshal([]byte(raw), &p)
		parsed, err := time.Parse(time.RFC3339, p.TS)
		if err != nil {
			t.Fatalf("timestamp parse failed: %v", err)
		}
		if parsed.Year() != 2025 || parsed.Month() != 6 || parsed.Day() != 15 {
			t.Errorf("parsed date = %v, want 2025-06-15", parsed)
		}
	})

	t.Run("roundtrip marshal/unmarshal", func(t *testing.T) {
		original := TrafficPayload{
			TS:        "2025-01-01T00:00:00Z",
			SensorID:  "SENSOR-01",
			RoadID:    "CITY-CENTER-01",
			SpeedKMH:  35.0,
			FlowRate:  200.0,
			Occupancy: 0.65,
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var decoded TrafficPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if decoded != original {
			t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
		}
	})
}
