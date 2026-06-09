package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	secret := "a,5W|/&?DThQv(r?m:4423nB}BuiXX@ZVhFa9fKd6(ts8{v<ppP#ZtY7^%L_n410"
	claims := jwt.MapClaims{
		"sub": "00000000-0000-0000-0000-000000000001",
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))

	fmt.Println(tokenString)
}
