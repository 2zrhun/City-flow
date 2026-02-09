package models

import "time"

type TrafficRaw struct {
	TS        time.Time `gorm:"column:ts;primaryKey" json:"ts"`
	SensorID  string    `gorm:"column:sensor_id;primaryKey" json:"sensor_id"`
	RoadID    string    `gorm:"column:road_id" json:"road_id"`
	SpeedKMH  float64   `gorm:"column:speed_kmh" json:"speed_kmh"`
	FlowRate  float64   `gorm:"column:flow_rate" json:"flow_rate"`
	Occupancy float64   `gorm:"column:occupancy" json:"occupancy"`
}

func (TrafficRaw) TableName() string { return "traffic_raw" }
