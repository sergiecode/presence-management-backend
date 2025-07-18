package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/mail.v2"
)

// Location represents a structured address
type Location struct {
	Calle        string       `json:"calle,omitempty"`
	Numero       string       `json:"numero,omitempty"`
	Piso         string       `json:"piso,omitempty"`
	Ciudad       string       `json:"ciudad,omitempty"`
	Provincia    string       `json:"provincia,omitempty"`
	CodigoPostal string       `json:"codigo_postal,omitempty"`
	Pais         string       `json:"pais,omitempty"`
	Tipo         LocationType `json:"tipo,omitempty" gorm:"type:int"`
}

func (l *Location) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal Location value: %v", value)
	}
	return json.Unmarshal(bytes, l)
}

func (l Location) Value() (driver.Value, error) {
	return json.Marshal(l)
}

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
	Active                bool      `json:"active" gorm:"default:true"`
	PasswordHash          string    `json:"-" gorm:"type:varchar(255)"`
	EmailConfirmed        bool      `json:"email_confirmed" gorm:"default:false"`
	ConfirmationToken     string    `json:"-" gorm:"type:varchar(255)"`
	ResetToken            string    `json:"-" gorm:"type:varchar(255)"`
	ResetTokenExpiry      time.Time `json:"-" gorm:"type:timestamp"`
	Surname               string    `json:"surname"`
	Phone                 string    `json:"phone"`
	CheckoutEndTime       string    `json:"checkout_end_time" gorm:"type:varchar(8);default:''"`

	// Additional fields from Excel files (optional, filled by HR)
	DNI                      string     `json:"dni,omitempty" gorm:"type:varchar(20)"`                       // National ID
	CUIL                     string     `json:"cuil,omitempty" gorm:"type:varchar(20)"`                      // Tax ID
	BirthDate                *time.Time `json:"birth_date,omitempty" gorm:"type:date"`                       // Birth date
	HireDate                 *time.Time `json:"hire_date,omitempty" gorm:"type:date"`                        // Hire date
	Location                 Location   `json:"location,omitempty" gorm:"type:jsonb"`                        // Structured address as JSON string
	WeeklyHours              int        `json:"weekly_hours,omitempty" gorm:"default:0"`                     // Weekly working hours
	Notes                    string     `json:"notes,omitempty" gorm:"type:text"`                            // General notes/aclaraciones
	Team                     string     `json:"team,omitempty" gorm:"type:varchar(50)"`                      // Team/Equipo
	ZohoAccess               bool       `json:"zoho_access,omitempty" gorm:"default:false"`                  // ZOHO access
	TeamsAccess              bool       `json:"teams_access,omitempty" gorm:"default:false"`                 // Teams access
	OnSiteRequired           bool       `json:"on_site_required,omitempty" gorm:"default:false"`             // Presencial requirement
	WeeklyObjectiveDays      int        `json:"weekly_objective_days,omitempty" gorm:"default:0"`            // Dias Objetivo SEMANA
	MonthlyObjectiveDays     int        `json:"monthly_objective_days,omitempty" gorm:"default:0"`           // Dias Objetivo MES
	OfficeDays               string     `json:"office_days,omitempty" gorm:"type:varchar(50)"`               // Office days (e.g., "MA/JU", "LU/MI")
	EndOfDayAbsenceDetection bool       `json:"end_of_day_absence_detection,omitempty" gorm:"default:false"` // Auto-detect absences at end of day
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

type LocationType int

const (
	LocationDomicilioRemotoDeclarado   LocationType = 1
	LocationDomicilioRemotoAlternativo LocationType = 2
	LocationDomicilioCliente           LocationType = 3
	LocationOficinaABSTI               LocationType = 4
)
