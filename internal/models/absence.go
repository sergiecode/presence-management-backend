package models

import "time"

type Absence struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Date      string    `json:"date" gorm:"type:date;index"`  // YYYY-MM-DD
	Type      string    `json:"type" gorm:"type:varchar(20)"` // absence/late/medical
	Reason    string    `json:"reason"`
	FileURL   string    `json:"file_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Locked    bool      `json:"locked" gorm:"default:false"`
	Deleted   bool      `json:"deleted" gorm:"default:false"`
}

type AbsenceRequest struct {
	Date   string `json:"date" binding:"required,datetime=2006-01-02"`
	Type   string `json:"type" binding:"required,oneof=absence late medical"`
	Reason string `json:"reason" binding:"required"`
}

type AbsenceResponse struct {
	ID        uint   `json:"id"`
	UserID    uint   `json:"user_id"`
	Date      string `json:"date"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	FileURL   string `json:"file_url,omitempty"`
	CreatedAt string `json:"created_at"`
}

type LockAbsenceRequest struct {
	Locked bool `json:"locked"`
}
