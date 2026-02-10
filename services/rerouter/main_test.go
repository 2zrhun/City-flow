package main

import (
	"os"
	"testing"
)

func TestAdjacencyMap(t *testing.T) {
	expectedRoads := []string{
		"RING-NORTH-12", "RING-SOUTH-09", "CITY-CENTER-01",
		"AIRPORT-AXIS-03", "UNIVERSITY-LOOP-07",
	}

	for _, road := range expectedRoads {
		alts, ok := adjacencyMap[road]
		if !ok {
			t.Errorf("road %q missing from adjacencyMap", road)
			continue
		}
		if len(alts) == 0 {
			t.Errorf("road %q has no alternatives", road)
		}
	}
}

func TestAdjacencyMapAlternativesExist(t *testing.T) {
	for road, alts := range adjacencyMap {
		for _, alt := range alts {
			if _, ok := adjacencyMap[alt]; !ok {
				t.Errorf("road %q lists %q as alternative, but %q is not in adjacencyMap", road, alt, alt)
			}
			if alt == road {
				t.Errorf("road %q lists itself as alternative", road)
			}
		}
	}
}

func TestRerouteDecisionLogic(t *testing.T) {
	threshold := 0.5

	tests := []struct {
		name        string
		scores      map[string]float64
		wantReroute bool
		wantFrom    string
		wantTo      string
	}{
		{
			name: "congested road with better alternative",
			scores: map[string]float64{
				"RING-NORTH-12": 0.8,
				"RING-SOUTH-09": 0.3,
				"CITY-CENTER-01": 0.4,
			},
			wantReroute: true,
			wantFrom:    "RING-NORTH-12",
			wantTo:      "RING-SOUTH-09",
		},
		{
			name: "no roads above threshold",
			scores: map[string]float64{
				"RING-NORTH-12": 0.3,
				"RING-SOUTH-09": 0.2,
			},
			wantReroute: false,
		},
		{
			name: "congested but alternatives also congested (delta < 0.1)",
			scores: map[string]float64{
				"RING-NORTH-12": 0.8,
				"RING-SOUTH-09": 0.75,
				"CITY-CENTER-01": 0.78,
			},
			wantReroute: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reroutes []Reroute
			for roadID, score := range tt.scores {
				if score <= threshold {
					continue
				}
				alternatives, ok := adjacencyMap[roadID]
				if !ok {
					continue
				}
				bestAlt := ""
				bestAltScore := 1.0
				for _, alt := range alternatives {
					altScore, exists := tt.scores[alt]
					if !exists {
						continue
					}
					if altScore < bestAltScore {
						bestAlt = alt
						bestAltScore = altScore
					}
				}
				if bestAlt == "" {
					continue
				}
				delta := score - bestAltScore
				if delta < 0.1 {
					continue
				}
				etaGain := delta * 15.0
				co2Gain := delta * 2.5
				reroutes = append(reroutes, Reroute{
					RouteID:          roadID,
					AltRouteID:       bestAlt,
					EstimatedCO2Gain: &co2Gain,
					ETAGainMin:       &etaGain,
				})
			}

			gotReroute := len(reroutes) > 0
			if gotReroute != tt.wantReroute {
				t.Errorf("reroute generated = %v, want %v", gotReroute, tt.wantReroute)
			}

			if tt.wantReroute && gotReroute {
				found := false
				for _, r := range reroutes {
					if r.RouteID == tt.wantFrom && r.AltRouteID == tt.wantTo {
						found = true
						if r.EstimatedCO2Gain == nil || *r.EstimatedCO2Gain <= 0 {
							t.Error("CO2 gain should be positive")
						}
						if r.ETAGainMin == nil || *r.ETAGainMin <= 0 {
							t.Error("ETA gain should be positive")
						}
					}
				}
				if !found {
					t.Errorf("expected reroute from %q to %q not found in %+v", tt.wantFrom, tt.wantTo, reroutes)
				}
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	os.Unsetenv("TEST_REROUTER_VAR")
	if got := getEnv("TEST_REROUTER_VAR", "default"); got != "default" {
		t.Errorf("getEnv() = %q, want %q", got, "default")
	}

	os.Setenv("TEST_REROUTER_VAR", "val")
	defer os.Unsetenv("TEST_REROUTER_VAR")
	if got := getEnv("TEST_REROUTER_VAR", "default"); got != "val" {
		t.Errorf("getEnv() = %q, want %q", got, "val")
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Unsetenv("TEST_REROUTER_INT")
	if got := getEnvInt("TEST_REROUTER_INT", 60); got != 60 {
		t.Errorf("getEnvInt() = %d, want %d", got, 60)
	}

	os.Setenv("TEST_REROUTER_INT", "120")
	defer os.Unsetenv("TEST_REROUTER_INT")
	if got := getEnvInt("TEST_REROUTER_INT", 60); got != 120 {
		t.Errorf("getEnvInt() = %d, want %d", got, 120)
	}
}

func TestGetEnvFloat(t *testing.T) {
	os.Unsetenv("TEST_REROUTER_FLOAT")
	if got := getEnvFloat("TEST_REROUTER_FLOAT", 0.5); got != 0.5 {
		t.Errorf("getEnvFloat() = %f, want %f", got, 0.5)
	}

	os.Setenv("TEST_REROUTER_FLOAT", "0.75")
	defer os.Unsetenv("TEST_REROUTER_FLOAT")
	if got := getEnvFloat("TEST_REROUTER_FLOAT", 0.5); got != 0.75 {
		t.Errorf("getEnvFloat() = %f, want %f", got, 0.75)
	}

	os.Setenv("TEST_REROUTER_FLOAT", "invalid")
	if got := getEnvFloat("TEST_REROUTER_FLOAT", 0.5); got != 0.5 {
		t.Errorf("getEnvFloat() with invalid should return fallback, got %f", got)
	}
}

func TestRoadPredictionStruct(t *testing.T) {
	rp := RoadPrediction{RoadID: "RING-NORTH-12", CongestionScore: 0.65}
	if rp.RoadID != "RING-NORTH-12" {
		t.Errorf("RoadID = %q", rp.RoadID)
	}
	if rp.CongestionScore != 0.65 {
		t.Errorf("CongestionScore = %v", rp.CongestionScore)
	}
}
