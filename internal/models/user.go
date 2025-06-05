package models

type User struct {
	ID uint `json:"id"`
	// ID        uint      `gorm:"primaryKey"` // add manually if you want it in docs
	// CreatedAt time.Time // add manually if you want it in docs
	// UpdatedAt time.Time // add manually if you want it in docs
	// DeletedAt gorm.DeletedAt `gorm:"index"` // add manually if you want it in docs
	Email                 string `gorm:"uniqueIndex" json:"email"`
	Name                  string `json:"name"`
	Picture               string `json:"picture"`
	Role                  string `json:"role" gorm:"default:employee"`
	CheckinStartTime      string `json:"checkin_start_time" gorm:"type:varchar(8);default:''"`
	Timezone              string `json:"timezone" gorm:"type:varchar(64);default:''"`
	NotificationOffsetMin int    `json:"notification_offset_min" gorm:"default:10"` // minutes before check-in
	PendingApproval       bool   `json:"pending_approval" gorm:"default:true"`
	Deactivated           bool   `json:"deactivated" gorm:"default:false"`
}
