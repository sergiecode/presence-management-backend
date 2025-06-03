package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"BE-ABSTI-CLOCKIN/internal/db"
	"BE-ABSTI-CLOCKIN/internal/models"
	"BE-ABSTI-CLOCKIN/pkg/jwtutil"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var oauthConf *oauth2.Config

func InitOAuthConfig() {
	oauthConf = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URI"),
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func RegisterAuthRoutes(r *gin.Engine) {
	InitOAuthConfig()
	r.GET("/auth/google/login", googleLoginHandler)
	r.GET("/auth/google/callback", googleCallbackHandler)
}

func googleLoginHandler(c *gin.Context) {
	url := oauthConf.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func googleCallbackHandler(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No code in request"})
		return
	}
	token, err := oauthConf.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token exchange failed", "details": err.Error()})
		return
	}
	client := oauthConf.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get userinfo", "details": err.Error()})
		return
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode userinfo", "details": err.Error()})
		return
	}

	var user models.User
	db.DB.Where(models.User{Email: userInfo.Email}).FirstOrInit(&user)
	user.Email = userInfo.Email
	user.Name = userInfo.Name
	user.Picture = userInfo.Picture
	if user.Role == "" {
		user.Role = "employee"
	}
	db.DB.Save(&user)

	jwtToken, err := jwtutil.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate JWT"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":   jwtToken,
		"email":   user.Email,
		"name":    user.Name,
		"picture": user.Picture,
		"id":      user.ID,
		"role":    user.Role,
	})
}
