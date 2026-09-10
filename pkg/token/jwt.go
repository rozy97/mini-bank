// Package token issues and verifies the HS256 JWTs used to authenticate API
// requests.
package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl}
}

func (m *JWTManager) Generate(userID int64, email string) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.ttl)
	c := claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// Verify returns the user ID embedded in tokenString, or an error if the
// token is malformed, unsigned by this manager's secret, or expired.
func (m *JWTManager) Verify(tokenString string) (int64, error) {
	var c claims
	token, err := jwt.ParseWithClaims(tokenString, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.secret, nil
	})
	if err != nil {
		return 0, err
	}
	if !token.Valid {
		return 0, jwt.ErrTokenInvalidClaims
	}
	return c.UserID, nil
}
