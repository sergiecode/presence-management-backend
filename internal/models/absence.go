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

type RegisterRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"secret123"`
	Name     string `json:"name" example:"John"`
	Surname  string `json:"surname" example:"Doe"`
	Phone    string `json:"phone" example:"+123456789"`
}

type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"secret123"`
}

type LoginResponse struct {
	Token   string `json:"token" example:"jwt.token.here"`
	Email   string `json:"email" example:"user@example.com"`
	Name    string `json:"name" example:"John"`
	Picture string `json:"picture" example:"https://example.com/avatar.jpg"`
	ID      uint   `json:"id" example:"1"`
	Role    string `json:"role" example:"admin"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// For /auth/forgot
// { "email": "user@example.com" }
type ForgotRequest struct {
	Email string `json:"email"`
}

// For /auth/reset
// { "token": "...", "new_password": "..." }
type ResetRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// For /auth/resend-confirmation
// { "email": "user@example.com" }
type ResendConfirmationRequest struct {
	Email string `json:"email"`
}

// For /auth/logout
// { "refresh_token": "..." }
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}
