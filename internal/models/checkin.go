package models

import "time"

// Checkin is the DB model for a user check-in
// GORM will auto-manage ID, CreatedAt, UpdatedAt
// UserID is a foreign key to User
// Date is stored as YYYY-MM-DD string for simplicity (could use time.Time if you want)
type Checkin struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	UserID         uint      `json:"user_id" gorm:"index"`
	Date           string    `json:"date" gorm:"type:date;index"`               // YYYY-MM-DD
	Time           time.Time `json:"time" gorm:"type:timestamp with time zone"` // precise check-in time
	LocationType   string    `json:"location_type"`
	LocationDetail string    `json:"location_detail,omitempty"`
	GPSLat         float64   `json:"gps_lat,omitempty"`
	GPSLong        float64   `json:"gps_long,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	Late           bool      `json:"late"`
	LateReason     string    `json:"late_reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Deleted        bool      `json:"deleted" gorm:"default:false"`
}

// NOTE: You must run a DB migration to convert the column type if you have existing data.

// CheckinRequest is what the mobile app sends
// Date is required, but backend should default to today if not provided
// LocationType is required: home, office, client, temporary
// GPS fields are optional
// Notes is optional
// LateReason is optional, only required if late
// Time is optional, backend will set if not provided
type CheckinRequest struct {
	Date           string  `json:"date" binding:"required,datetime=2006-01-02"`
	Time           string  `json:"time,omitempty"` // still accept string for backward compatibility
	LocationType   string  `json:"location_type" binding:"required,oneof=home office client temporary"`
	LocationDetail string  `json:"location_detail,omitempty"`
	GPSLat         float64 `json:"gps_lat,omitempty"`
	GPSLong        float64 `json:"gps_long,omitempty"`
	Notes          string  `json:"notes,omitempty"`
	LateReason     string  `json:"late_reason,omitempty"`
}

// CheckinResponse is what the API returns
// Mirrors the Checkin model, but can be extended for extra info
// (e.g., user info, status, etc.)
type CheckinResponse struct {
	ID             uint    `json:"id"`
	UserID         uint    `json:"user_id"`
	Date           string  `json:"date"`
	Time           string  `json:"time"` // return as RFC3339 string for API clients
	LocationType   string  `json:"location_type"`
	LocationDetail string  `json:"location_detail,omitempty"`
	GPSLat         float64 `json:"gps_lat,omitempty"`
	GPSLong        float64 `json:"gps_long,omitempty"`
	Notes          string  `json:"notes,omitempty"`
	Late           bool    `json:"late"`
	LateReason     string  `json:"late_reason,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

// CheckinConfigRequest is what the mobile app sends
// CheckinStartTime is optional, backend will set if not provided
// Timezone is optional, backend will set if not provided
// NotificationOffsetMin is optional, backend will set if not provided
type CheckinConfigRequest struct {
	CheckinStartTime      string `json:"checkin_start_time"`
	Timezone              string `json:"timezone"`
	NotificationOffsetMin int    `json:"notification_offset_min"`
}
