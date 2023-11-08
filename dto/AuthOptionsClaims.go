package dto

import "github.com/golang-jwt/jwt/v5"

type AuthOptionsClaims struct {
	jwt.RegisteredClaims
	Client       string `form:"client" json:"client" binding:"required"`
	ClientUserId string `form:"client_user_id" json:"clientUserId"  binding:"required"`
	RedirectUri  string `form:"redirect_uri" json:"redirectUri,omitempty"`
	KneuUserId   uint   `form:"-" json:"userId,omitempty"`
}
