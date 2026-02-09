package models

import "time"

type Prediction struct {
	TS              time.Time `gorm:"column:ts;primaryKey" json:"ts"`
	RoadID          string    `gorm:"column:road_id;primaryKey" json:"road_id"`
	HorizonMin      int       `gorm:"column:horizon_min;primaryKey;default:30" json:"horizon_min"`
	CongestionScore float64   `gorm:"column:congestion_score" json:"congestion_score"`
	Confidence      *float64  `gorm:"column:confidence" json:"confidence"`
	ModelVersion    string    `gorm:"column:model_version" json:"model_version"`
}

func (Prediction) TableName() string { return "predictions" }
