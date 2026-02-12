package models

import "time"

type Road struct {
	RoadID    string    `gorm:"column:road_id;primaryKey" json:"road_id"`
	Label     string    `gorm:"column:label" json:"label"`
	Lat       *float64  `gorm:"column:lat" json:"lat"`
	Lng       *float64  `gorm:"column:lng" json:"lng"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Road) TableName() string { return "roads" }
