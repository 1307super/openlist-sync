package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/openlist-sync/internal/auth"
	"github.com/user/openlist-sync/internal/database"
)

func AuthMiddleware(db *sql.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if strings := splitAuthHeader(token); strings != "" {
			token = strings
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
			return
		}

		claims, valid := auth.ValidateToken(token, secret)
		if !valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "登录已过期"})
			return
		}

		c.Set("authUser", claims.Username)
		c.Next()
	}
}

func splitAuthHeader(header string) string {
	if len(header) > 7 && header[:7] == "Bearer " {
		return header[7:]
	}
	return ""
}

func LoginHandler(db *sql.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}

		savedUser, _ := database.GetSetting(db, "auth_username")
		savedHash, _ := database.GetSetting(db, "auth_password")

		if savedUser == "" {
			savedUser = "admin"
		}

		if body.Username != savedUser {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户名或密码错误"})
			return
		}

		if !auth.CheckPassword(body.Password, savedHash) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户名或密码错误"})
			return
		}

		token := auth.GenerateToken(savedUser, secret)
		c.JSON(http.StatusOK, gin.H{"token": token, "username": savedUser})
	}
}

func InitDefaultCredentials(db *sql.DB) {
	savedUser, _ := database.GetSetting(db, "auth_username")
	if savedUser == "" {
		database.UpsertSetting(db, "auth_username", "admin")
	}
	savedPass, _ := database.GetSetting(db, "auth_password")
	if savedPass == "" {
		database.UpsertSetting(db, "auth_password", auth.HashPassword("admin"))
	}
}
