package models

import "time"

// AttendanceRow is used for scanning attendance query results
// All fields except UserID are pointers so null is possible
// This struct is used internally in the dashboard handler

type AttendanceRow struct {
	UserID           uint
	Name             string
	Email            string
	CheckinID        *uint
	CheckinTime      *time.Time
	Late             *bool
	LocationType     *int
	LocationDetail   *string
	Notes            *string
	LateReason       *string
	CheckinCreatedAt *time.Time
	AbsenceID        *uint
	AbsenceType      *int
	AbsenceReason    *string
	FileURL          *string
	AbsenceCreatedAt *time.Time
	CheckoutTime     *time.Time
	CheckoutStatus   *string
	Overtime         *bool
}

// AttendanceResponse is used for dashboard attendance API responses
// All fields except UserID are pointers so null is possible, and all fields are always present in JSON

type AttendanceResponse struct {
	UserID           uint              `json:"user_id"`
	Name             *string           `json:"name"`
	Email            *string           `json:"email"`
	CheckinID        *uint             `json:"checkin_id"`
	CheckinTime      *time.Time        `json:"checkin_time"`
	Late             *bool             `json:"late"`
	LocationType     *int              `json:"location_type"`
	LocationDetail   *string           `json:"location_detail"`
	Locations        []CheckinLocation `json:"locations,omitempty"`
	Notes            *string           `json:"notes"`
	LateReason       *string           `json:"late_reason"`
	CheckinCreatedAt *time.Time        `json:"checkin_created_at"`
	AbsenceID        *uint             `json:"absence_id"`
	AbsenceType      *int              `json:"absence_type"`
	AbsenceReason    *string           `json:"absence_reason"`
	FileURL          *string           `json:"file_url"`
	AbsenceCreatedAt *time.Time        `json:"absence_created_at"`
	CheckoutTime     *time.Time        `json:"checkout_time"`
	CheckoutStatus   *string           `json:"checkout_status"`
	Overtime         *bool             `json:"overtime"`
}

// ToAttendanceResponse converts an AttendanceRow to AttendanceResponse
func ToAttendanceResponse(row AttendanceRow) AttendanceResponse {
	response := AttendanceResponse{
		UserID:           row.UserID,
		Name:             &row.Name,
		Email:            &row.Email,
		CheckinID:        row.CheckinID,
		CheckinTime:      row.CheckinTime,
		Late:             row.Late,
		LocationType:     row.LocationType,
		LocationDetail:   row.LocationDetail,
		Notes:            row.Notes,
		LateReason:       row.LateReason,
		CheckinCreatedAt: row.CheckinCreatedAt,
		AbsenceID:        row.AbsenceID,
		AbsenceType:      row.AbsenceType,
		AbsenceReason:    row.AbsenceReason,
		FileURL:          row.FileURL,
		AbsenceCreatedAt: row.AbsenceCreatedAt,
		CheckoutTime:     row.CheckoutTime,
		CheckoutStatus:   row.CheckoutStatus,
		Overtime:         row.Overtime,
	}

	// Initialize empty locations array
	response.Locations = []CheckinLocation{}

	return response
}

// ExportCheckinRow is used for exporting checkin data to Excel or JSON
// All fields except Empleado, DNI, CUIL, Date, Time are pointers for nullability

type ExportCheckinRow struct {
	Empleado       *string    `json:"empleado"`
	DNI            *string    `json:"dni"`
	CUIL           *string    `json:"cuil"`
	BirthDate      *time.Time `json:"birth_date"`
	HireDate       *time.Time `json:"hire_date"`
	Location       *string    `json:"location"`
	HrsSemanales   *int       `json:"hrs_semanales"`
	Aclaraciones   *string    `json:"aclaraciones"`
	Date           *string    `json:"date"`
	Time           *time.Time `json:"time"`
	LocationType   *int       `json:"location_type"`
	LocationDetail *string    `json:"location_detail"`
	Late           *bool      `json:"late"`
	LateReason     *string    `json:"late_reason"`
	CheckoutTime   *time.Time `json:"checkout_time"`
	CheckoutStatus *string    `json:"checkout_status"`
	Overtime       *bool      `json:"overtime"`
}

// ExportAttendanceRow is used for exporting attendance data to Excel or JSON
// All fields except UserID are pointers for nullability

type ExportAttendanceRow struct {
	UserID         uint       `json:"user_id"`
	Empleado       *string    `json:"empleado"`
	DNI            *string    `json:"dni"`
	CUIL           *string    `json:"cuil"`
	BirthDate      *time.Time `json:"birth_date"`
	HireDate       *time.Time `json:"hire_date"`
	Location       *string    `json:"location"`
	HrsSemanales   *int       `json:"hrs_semanales"`
	Aclaraciones   *string    `json:"aclaraciones"`
	CheckinID      *uint      `json:"checkin_id"`
	CheckinTime    *time.Time `json:"checkin_time"`
	Late           *bool      `json:"late"`
	LocationType   *int       `json:"location_type"`
	LocationDetail *string    `json:"location_detail"`
	Notes          *string    `json:"notes"`
	LateReason     *string    `json:"late_reason"`
	CheckoutTime   *time.Time `json:"checkout_time"`
	CheckoutStatus *string    `json:"checkout_status"`
	Overtime       *bool      `json:"overtime"`
	AbsenceID      *uint      `json:"absence_id"`
	AbsenceType    *int       `json:"absence_type"`
	AbsenceReason  *string    `json:"absence_reason"`
}

// IndividualAttendanceResponse is used for the getIndividualAttendance endpoint
// All fields are pointers for nullability and consistency

type IndividualAttendanceResponse struct {
	User     *UserResponse     `json:"user"`
	Checkin  *CheckinResponse  `json:"checkin"`
	Checkout *CheckoutResponse `json:"checkout"`
	Absence  *AbsenceResponse  `json:"absence"`
}

type CheckoutResponse struct {
	CheckoutTime   *time.Time `json:"checkout_time"`
	CheckoutStatus *string    `json:"checkout_status"`
	Overtime       *bool      `json:"overtime"`
}

// ToIndividualAttendanceResponse builds the response from the models
func ToIndividualAttendanceResponse(user *User, checkin *Checkin, absence *Absence) IndividualAttendanceResponse {
	var userResp *UserResponse
	if user != nil {
		ur := ToUserResponse(*user)
		userResp = &ur
	}
	var checkinResp *CheckinResponse
	var checkoutResp *CheckoutResponse
	if checkin != nil {
		cr := CheckinResponse{
			ID:             checkin.ID,
			UserID:         checkin.UserID,
			Time:           checkin.Time.Format(time.RFC3339), // Return UTC time with Z
			Notes:          checkin.Notes,
			Late:           checkin.Late,
			LateReason:     checkin.LateReason,
			CreatedAt:      checkin.CreatedAt.Format(time.RFC3339),
			CheckoutTime:   checkin.CheckoutTime,
			CheckoutStatus: checkin.CheckoutStatus,
			Overtime:       checkin.Overtime,
			AbsenceID:      checkin.AbsenceID,
			Locations:      checkin.Locations,
		}
		checkinResp = &cr
		if checkin.CheckoutTime != nil || checkin.CheckoutStatus != "" || checkin.Overtime {
			checkoutResp = &CheckoutResponse{
				CheckoutTime:   checkin.CheckoutTime,
				CheckoutStatus: &checkin.CheckoutStatus,
				Overtime:       &checkin.Overtime,
			}
		}
	}
	var absenceResp *AbsenceResponse
	if absence != nil {
		ar := AbsenceResponse{
			ID:        absence.ID,
			UserID:    absence.UserID,
			Date:      absence.Date,
			Type:      absence.Type,
			Reason:    absence.Reason,
			FileURL:   absence.FileURL,
			CreatedAt: absence.CreatedAt.Format(time.RFC3339),
		}
		absenceResp = &ar
	}
	return IndividualAttendanceResponse{
		User:     userResp,
		Checkin:  checkinResp,
		Checkout: checkoutResp,
		Absence:  absenceResp,
	}
}

// TeamUsersResponse is used for getUsersByTeam endpoint

type TeamUsersResponse struct {
	Team  string         `json:"team"`
	Users []UserResponse `json:"users"`
}

// ToTeamUsersResponse converts a team name and []User to TeamUsersResponse
func ToTeamUsersResponse(team string, users []User) TeamUsersResponse {
	userResponses := make([]UserResponse, len(users))
	for i, u := range users {
		userResponses[i] = ToUserResponse(u)
	}
	return TeamUsersResponse{
		Team:  team,
		Users: userResponses,
	}
}
