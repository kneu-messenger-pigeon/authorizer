package dto

import "github.com/kneu-messenger-pigeon/events"

type Student struct {
	Id         uint
	LastName   string
	FirstName  string
	MiddleName string
	Gender     events.Gender
}
