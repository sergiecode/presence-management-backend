package models

import "time"

type AuditLog struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	UserEmail  string    `json:"user_email"`  // Who made the change (HR/admin)
	Action     string    `json:"action"`      // e.g. "update_checkin"
	EntityID   uint      `json:"entity_id"`   // e.g. checkin ID
	EntityType string    `json:"entity_type"` // e.g. "checkin"
	OldValue   string    `json:"old_value"`   // JSON string of old checkin
	NewValue   string    `json:"new_value"`   // JSON string of new checkin
	Timestamp  time.Time `json:"timestamp"`
}

type AuditLogListResponse struct {
	Logs     []AuditLog `json:"logs"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}
