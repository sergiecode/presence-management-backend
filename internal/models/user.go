package models

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/mail.v2"
)

type User struct {
	ID uint `json:"id"`
	// ID        uint      `gorm:"primaryKey"` // add manually if you want it in docs
	// CreatedAt time.Time // add manually if you want it in docs
	// UpdatedAt time.Time // add manually if you want it in docs
	// DeletedAt gorm.DeletedAt `gorm:"index"` // add manually if you want it in docs
	Email                 string    `gorm:"uniqueIndex" json:"email"`
	Name                  string    `json:"name"`
	Picture               string    `json:"picture"`
	Role                  string    `json:"role" gorm:"default:employee"`
	CheckinStartTime      string    `json:"checkin_start_time" gorm:"type:varchar(8);default:''"`
	Timezone              string    `json:"timezone" gorm:"type:varchar(64);default:''"`
	NotificationOffsetMin int       `json:"notification_offset_min" gorm:"default:10"` // minutes before check-in
	PendingApproval       bool      `json:"pending_approval" gorm:"default:true"`
	Deactivated           bool      `json:"deactivated" gorm:"default:false"`
	PasswordHash          string    `json:"-" gorm:"type:varchar(255)"`
	EmailConfirmed        bool      `json:"email_confirmed" gorm:"default:false"`
	ConfirmationToken     string    `json:"-" gorm:"type:varchar(255)"`
	ResetToken            string    `json:"-" gorm:"type:varchar(255)"`
	ResetTokenExpiry      time.Time `json:"-" gorm:"type:timestamp"`
	Surname               string    `json:"surname"`
	Phone                 string    `json:"phone"`
	CheckoutEndTime       string    `json:"checkout_end_time" gorm:"type:varchar(8);default:''"`
}

type RefreshToken struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index"`
	Token     string `gorm:"uniqueIndex"`
	ExpiresAt time.Time
	Revoked   bool `gorm:"default:false"`
	CreatedAt time.Time
}

func SendMail(to, subject, body string) error {
	m := mail.NewMessage()
	m.SetHeader("From", os.Getenv("EMAIL_FROM"))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	host := os.Getenv("EMAIL_HOST")
	port, _ := strconv.Atoi(os.Getenv("EMAIL_PORT"))
	user := os.Getenv("EMAIL_USER")
	pass := os.Getenv("EMAIL_PASSWORD")
	d := mail.NewDialer(host, port, user, pass)
	return d.DialAndSend(m)
}
