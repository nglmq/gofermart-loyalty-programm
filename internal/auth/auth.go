package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

const TokenExp = time.Hour * 3
const SecretKey = "supersecretkey"

type Claims struct {
	jwt.RegisteredClaims
	Login string
}

func BuildJWTString(login string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExp)),
		},
		Login: login,
	})

	tokenString, err := token.SignedString([]byte(SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func GetUserID(tokenString string) string {

	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(SecretKey), nil
	})
	if err != nil {
		return ""
	}

	if !token.Valid {
		fmt.Println("token is not valid")
		return ""
	}

	return claims.Login
}
