package models

import "time"

type Reroute struct {
	TS               time.Time `gorm:"column:ts;primaryKey" json:"ts"`
	RouteID          string    `gorm:"column:route_id;primaryKey" json:"route_id"`
	AltRouteID       string    `gorm:"column:alt_route_id;primaryKey" json:"alt_route_id"`
	Reason           string    `gorm:"column:reason" json:"reason"`
	EstimatedCO2Gain *float64  `gorm:"column:estimated_co2_gain" json:"estimated_co2_gain"`
	ETAGainMin       *float64  `gorm:"column:eta_gain_min" json:"eta_gain_min"`
}

func (Reroute) TableName() string { return "reroutes" }
