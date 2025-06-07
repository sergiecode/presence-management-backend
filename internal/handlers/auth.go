package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"
	"BE-ABSTI-CLOCKIN/pkg/jwtutil"

	"crypto/rand"
	"encoding/base64"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mail.v2"
)

func RegisterAuthRoutes(r *gin.Engine) {
	r.POST("/auth/register", registerHandler)
	r.GET("/auth/confirm", confirmHandler)
	r.POST("/auth/login", loginHandler)
	r.POST("/auth/forgot", forgotHandler)
	r.POST("/auth/reset", resetHandler)
	r.POST("/auth/refresh", refreshHandler)
	r.POST("/auth/logout", logoutHandler)
	r.POST("/auth/resend-confirmation", resendConfirmationHandler)
}

// @Summary Register a new user
// @Description Register with name, surname, email, phone, and password. Sends confirmation email.
// @Tags auth
// @Accept json
// @Produce json
// @Param registration body models.RegisterRequest true "Registration data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /auth/register [post]
func registerHandler(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request", Details: err.Error()})
		return
	}
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Email and password are required"})
		return
	}
	// Check if user exists
	var existing models.User
	if err := db.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "User with this email already exists"})
		return
	}
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to hash password", Details: err.Error()})
		return
	}
	// Generate confirmation token
	token := genToken(32)
	user := models.User{
		Email:             req.Email,
		Name:              req.Name,
		PasswordHash:      string(hash),
		EmailConfirmed:    false,
		ConfirmationToken: token,
		PendingApproval:   true,
		Deactivated:       true,
	}
	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create user", Details: err.Error()})
		return
	}
	// Send confirmation email
	confirmURL := fmt.Sprintf("%s/auth/confirm?token=%s", getBaseURL(c), token)
	subject := "Confirm your email"
	body := fmt.Sprintf("<p>Welcome! Please confirm your email by clicking <a href=\"%s\">here</a>.</p>", confirmURL)
	if err := models.SendMail(user.Email, subject, body); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to send confirmation email", Details: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Registration successful. Please check your email to confirm your account."})
}

// Helper: generate cryptographically secure random token (for confirmation, reset, etc)
func genToken(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

// Helper: get base URL from request
func getBaseURL(c *gin.Context) string {
	proto := c.Request.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := c.Request.Host
	return fmt.Sprintf("%s://%s", proto, host)
}

// GET /auth/confirm?token=...
func confirmHandler(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing token"})
		return
	}
	var user models.User
	err := db.DB.Where("confirmation_token = ?", token).First(&user).Error
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired token"})
		return
	}
	if user.EmailConfirmed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already confirmed"})
		return
	}
	user.EmailConfirmed = true
	user.ConfirmationToken = ""
	user.PendingApproval = false
	user.Deactivated = false
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm email", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Email confirmed. You can now log in."})
}

// Helper: generate secure random string for refresh token
func genRefreshTokenString(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

// Helper: create and store refresh token in DB, limit to 5 active per user
func createRefreshToken(userID uint) (string, error) {
	token := genRefreshTokenString(32)
	expires := time.Now().Add(7 * 24 * time.Hour)
	rt := models.RefreshToken{
		UserID:    userID,
		Token:     token,
		ExpiresAt: expires,
		Revoked:   false,
		CreatedAt: time.Now(),
	}
	// Limit to 5 active refresh tokens per user
	var tokens []models.RefreshToken
	db.DB.Where("user_id = ? AND revoked = ? AND expires_at > ?", userID, false, time.Now()).Order("created_at asc").Find(&tokens)
	if len(tokens) >= 5 {
		// Revoke the oldest
		oldest := tokens[0]
		db.DB.Model(&oldest).Update("revoked", true)
	}
	if err := db.DB.Create(&rt).Error; err != nil {
		return "", err
	}
	return token, nil
}

// @Summary Login
// @Description Authenticate with email and password. Returns JWT on success.
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /auth/login [post]
func loginHandler(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	var user models.User
	if err := db.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	if !user.EmailConfirmed {
		c.JSON(http.StatusForbidden, gin.H{"error": "Email not confirmed"})
		return
	}
	if user.PendingApproval {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account pending HR approval"})
		return
	}
	if user.Deactivated {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account deactivated"})
		return
	}
	token, err := jwtutil.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate JWT"})
		return
	}
	refreshToken, err := createRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"role":          user.Role,
		"id":            user.ID,
	})
}

// POST /auth/forgot
func forgotHandler(c *gin.Context) {
	type reqBody struct {
		Email string `json:"email"`
	}
	var req reqBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	var user models.User
	if err := db.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal if user exists
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent."})
		return
	}
	token := genToken(32)
	expiry := time.Now().Add(1 * time.Hour)
	user.ResetToken = token
	user.ResetTokenExpiry = expiry
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set reset token"})
		return
	}
	resetURL := fmt.Sprintf("%s/auth/reset?token=%s", getBaseURL(c), token)
	subject := "Password Reset Request"
	body := fmt.Sprintf("<p>To reset your password, click <a href=\"%s\">here</a>. This link expires in 1 hour.</p>", resetURL)
	_ = models.SendMail(user.Email, subject, body) // Don't error leak
	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent."})
}

// POST /auth/reset
func resetHandler(c *gin.Context) {
	type reqBody struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	var req reqBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token and new password are required"})
		return
	}
	var user models.User
	if err := db.DB.Where("reset_token = ?", req.Token).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired token"})
		return
	}
	if user.ResetTokenExpiry.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reset token expired"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	user.PasswordHash = string(hash)
	user.ResetToken = ""
	user.ResetTokenExpiry = time.Time{}
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset. You can now log in."})
}

// @Summary Refresh JWT access token
// @Description Exchange a valid refresh token for a new access token. Rotates refresh token.
// @Tags auth
// @Accept json
// @Produce json
// @Param refresh body struct{RefreshToken string `json:"refresh_token"`} true "Refresh token"
// @Success 200 {object} map[string]string
// @Failure 401 {object} models.ErrorResponse
// @Router /auth/refresh [post]
func refreshHandler(c *gin.Context) {
	type reqBody struct {
		RefreshToken string `json:"refresh_token"`
	}
	var req reqBody
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.JSON(401, models.ErrorResponse{Error: "Invalid refresh token"})
		return
	}
	var rt models.RefreshToken
	if err := db.DB.Where("token = ? AND revoked = ?", req.RefreshToken, false).First(&rt).Error; err != nil {
		c.JSON(401, models.ErrorResponse{Error: "Invalid refresh token"})
		return
	}
	if rt.ExpiresAt.Before(time.Now()) {
		c.JSON(401, models.ErrorResponse{Error: "Refresh token expired"})
		return
	}
	// Optionally: rotate refresh token (revoke old, issue new)
	db.DB.Model(&rt).Update("revoked", true)
	newRefresh, err := createRefreshToken(rt.UserID)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to issue new refresh token"})
		return
	}
	// Issue new access token
	var user models.User
	if err := db.DB.First(&user, rt.UserID).Error; err != nil {
		c.JSON(401, models.ErrorResponse{Error: "User not found"})
		return
	}
	token, err := jwtutil.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		c.JSON(500, models.ErrorResponse{Error: "Failed to generate JWT"})
		return
	}
	c.JSON(200, gin.H{"token": token, "refresh_token": newRefresh})
}

// @Summary Logout (revoke refresh token)
// @Description Revoke a refresh token (logout from device/session)
// @Tags auth
// @Accept json
// @Produce json
// @Param logout body struct{RefreshToken string `json:"refresh_token"`} true "Refresh token"
// @Success 200 {object} map[string]string
// @Failure 401 {object} models.ErrorResponse
// @Router /auth/logout [post]
func logoutHandler(c *gin.Context) {
	type reqBody struct {
		RefreshToken string `json:"refresh_token"`
	}
	var req reqBody
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.JSON(401, models.ErrorResponse{Error: "Invalid refresh token"})
		return
	}
	db.DB.Model(&models.RefreshToken{}).Where("token = ?", req.RefreshToken).Update("revoked", true)
	c.JSON(200, gin.H{"message": "Logged out"})
}

// @Summary Resend confirmation email
// @Description Resend the email confirmation link to a user who hasn't confirmed yet.
// @Tags auth
// @Accept json
// @Produce json
// @Param email body struct{Email string `json:"email"`} true "User email"
// @Success 200 {object} map[string]string
// @Router /auth/resend-confirmation [post]
func resendConfirmationHandler(c *gin.Context) {
	type reqBody struct {
		Email string `json:"email"`
	}
	var req reqBody
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a confirmation link has been sent."})
		return
	}
	var user models.User
	if err := db.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal if user exists
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a confirmation link has been sent."})
		return
	}
	if user.EmailConfirmed {
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a confirmation link has been sent."})
		return
	}
	// Generate new token and save
	token := genToken(32)
	user.ConfirmationToken = token
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a confirmation link has been sent."})
		return
	}
	confirmURL := fmt.Sprintf("%s/auth/confirm?token=%s", getBaseURL(c), token)
	subject := "Confirm your email"
	body := fmt.Sprintf("<p>Please confirm your email by clicking <a href=\"%s\">here</a>.</p>", confirmURL)
	_ = models.SendMail(user.Email, subject, body)
	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a confirmation link has been sent."})
}

func SendMail(to, subject, body string) error {
	m := mail.NewMessage()
	m.SetHeader("From", os.Getenv("EMAIL_FROM"))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	host := os.Getenv("EMAIL_HOST")
	portStr := os.Getenv("EMAIL_PORT")
	if portStr == "" {
		portStr = "587"
	}
	port, _ := strconv.Atoi(portStr)
	user := os.Getenv("EMAIL_USER")
	pass := os.Getenv("EMAIL_PASSWORD")

	d := mail.NewDialer(host, port, user, pass)
	return d.DialAndSend(m)
}
