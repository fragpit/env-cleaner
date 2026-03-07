package model

type Notificator interface {
	SendOrphanMessage(env *Environment) error
	SendStaleMessage(env *Environment, tk *Token) error
	SendDeleteMessage(env *Environment) error
}
