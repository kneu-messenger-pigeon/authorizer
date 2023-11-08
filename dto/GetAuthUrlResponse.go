package dto

import "time"

type GetAuthUrlResponse struct {
	AuthUrl  string    `json:"authUrl" binding:"required"`
	ExpireAt time.Time `json:"expire,omitempty" binding:"required"`
}
