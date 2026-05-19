package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const tokenValidity = 7 * 24 * time.Hour

type TokenClaims struct {
	Username string
	Expires  int64
}

func GenerateToken(username, secret string) string {
	expires := time.Now().Add(tokenValidity).Unix()
	payload := fmt.Sprintf("%s|%d", username, expires)
	sig := sign(payload, secret)
	return fmt.Sprintf("%s|%s", payload, sig)
}

func ValidateToken(token, secret string) (*TokenClaims, bool) {
	parts := strings.SplitN(token, "|", 3)
	if len(parts) != 3 {
		return nil, false
	}

	username, expStr, sig := parts[0], parts[1], parts[2]
	payload := fmt.Sprintf("%s|%s", username, expStr)

	if !hmac.Equal([]byte(sign(payload, secret)), []byte(sig)) {
		return nil, false
	}

	expires, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return nil, false
	}

	if time.Now().Unix() > expires {
		return nil, false
	}

	return &TokenClaims{Username: username, Expires: expires}, true
}

func HashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

func CheckPassword(password, hash string) bool {
	return HashPassword(password) == hash
}

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
