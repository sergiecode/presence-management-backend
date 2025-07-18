package models

import "time"

// Checkin is the DB model for a user check-in
// GORM will auto-manage ID, CreatedAt, UpdatedAt
// UserID is a foreign key to User
// Time is the main check-in timestamp (replaces separate date field)
type Checkin struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	UserID         uint       `json:"user_id" gorm:"index"`
	Time           time.Time  `json:"time" gorm:"type:timestamp with time zone;index"` // Main check-in time
	Notes          string     `json:"notes,omitempty"`
	Late           bool       `json:"late"`
	LateReason     string     `json:"late_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Deleted        bool       `json:"deleted" gorm:"default:false"`
	CheckoutTime   *time.Time `json:"checkout_time,omitempty" gorm:"type:timestamp with time zone"`
	CheckoutStatus string     `json:"checkout_status,omitempty" gorm:"type:varchar(32);default:''"`
	Overtime       bool       `json:"overtime" gorm:"default:false"`
	AbsenceID      *uint      `json:"absence_id,omitempty" gorm:"index"`
	// Relationship to multiple locations
	Locations []CheckinLocation `json:"locations" gorm:"foreignKey:CheckinID"`
}

// CheckinLocation represents a work location for a check-in
// One check-in can have multiple locations (e.g., office in morning, client in afternoon)
type CheckinLocation struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	CheckinID      uint      `json:"checkin_id" gorm:"index"`
	LocationType   int       `json:"location_type" binding:"required,min=1,max=4"`
	LocationDetail string    `json:"location_detail,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// CheckinRequest is what the mobile app sends
// Time is required, backend will set if not provided
// Notes is optional
// LateReason is optional, only required if late
// Locations is an array of work locations for the day
type CheckinRequest struct {
	UserID     uint              `json:"user_id,omitempty"` // Optional, extracted from JWT if not provided
	Time       string            `json:"time,omitempty"`    // Optional, defaults to now
	Notes      string            `json:"notes,omitempty"`
	LateReason string            `json:"late_reason,omitempty"`
	Locations  []LocationRequest `json:"locations" binding:"required,min=1"`
}

// LocationRequest represents a single location in the check-in request
type LocationRequest struct {
	LocationType   int    `json:"location_type" binding:"required,min=1,max=4"`
	LocationDetail string `json:"location_detail,omitempty"`
}

// UpdateLocationsRequest is used to update locations during the day
type UpdateLocationsRequest struct {
	Locations []LocationRequest `json:"locations" binding:"required,min=1"`
}

// CheckinResponse is what the API returns
// Mirrors the Checkin model, but can be extended for extra info
// (e.g., user info, status, etc.)
type CheckinResponse struct {
	ID             uint              `json:"id"`
	UserID         uint              `json:"user_id"`
	Time           string            `json:"time"` // return as RFC3339 string for API clients
	Notes          string            `json:"notes,omitempty"`
	Late           bool              `json:"late"`
	LateReason     string            `json:"late_reason,omitempty"`
	CreatedAt      string            `json:"created_at"`
	CheckoutTime   *time.Time        `json:"checkout_time,omitempty"`
	CheckoutStatus string            `json:"checkout_status,omitempty"`
	Overtime       bool              `json:"overtime,omitempty"`
	AbsenceID      *uint             `json:"absence_id,omitempty"`
	Locations      []CheckinLocation `json:"locations"`
}

// CheckinConfigRequest is what the mobile app sends
// CheckinStartTime is optional, backend will set if not provided
// Timezone is optional, backend will set if not provided
// NotificationOffsetMin is optional, backend will set if not provided
type CheckinConfigRequest struct {
	CheckinStartTime      string `json:"checkin_start_time"`
	Timezone              string `json:"timezone"`
	NotificationOffsetMin int    `json:"notification_offset_min"`
	CheckoutEndTime       string `json:"checkout_end_time"`
}

// DailySummary matches the daily_summary table for reporting/aggregation
// Used for fast dashboard stats and analytics
type DailySummary struct {
	Date          string    `json:"date" gorm:"primaryKey"`
	TotalCheckins int       `json:"total_checkins"`
	TotalOnTime   int       `json:"total_on_time"`
	TotalLate     int       `json:"total_late"`
	TotalOvertime int       `json:"total_overtime"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Checkin) TableName() string {
	return "checkins"
}

// Add GORM index for (user_id, date)
// In migration: db.Model(&Checkin{}).AddIndex("idx_user_date", "user_id", "date")

// For /checkins/checkout (user checkout)
type CheckoutRequest struct {
	Status       string `json:"status"`
	Overtime     bool   `json:"overtime"`
	CheckoutTime string `json:"checkout_time,omitempty"` // RFC3339, optional
}

// For /checkins/checkout/:id (HR/admin update)
type CheckoutUpdateRequest struct {
	CheckoutTime   string `json:"checkout_time"`
	CheckoutStatus string `json:"checkout_status"`
	Overtime       bool   `json:"overtime"`
}

type MonthlyStat struct {
	Month    string `json:"month"`
	Total    int64  `json:"total"`
	Late     int64  `json:"late"`
	Overtime int64  `json:"overtime"`
}

// BatchApproveRequest is used for batch approval/rejection of checkins
type BatchApproveRequest struct {
	IDs    []uint `json:"ids"`
	Action string `json:"action"` // "approve" or "reject"
	Reason string `json:"reason"`
}

// BatchApproveResponse is optional, but you might want it for docs
type BatchApproveResponse struct {
	Success   bool   `json:"success"`
	Processed int    `json:"processed"`
	Failed    int    `json:"failed"`
	Message   string `json:"message"`
}

// AbsenceHeatmapEntry is used for /api/dashboard/analytics/heatmap
type AbsenceHeatmapEntry struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// OvertimeStatEntry is used for /api/dashboard/analytics/overtime
type OvertimeStatEntry struct {
	Date     string `json:"date"`
	Overtime int64  `json:"overtime"`
}

// ConvertAbsenceRequest is used for HR/admin to convert absence to checkin
type ConvertAbsenceRequest struct {
	AbsenceID   uint              `json:"absence_id" binding:"required"`
	CheckinTime string            `json:"checkin_time,omitempty"` // RFC3339, optional
	Notes       string            `json:"notes,omitempty"`
	Late        bool              `json:"late"`
	LateReason  string            `json:"late_reason,omitempty"`
	Locations   []LocationRequest `json:"locations" binding:"required,min=1"`
}
