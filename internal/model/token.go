package model

import "context"

type Token struct {
	EnvID string
	Token string
}

type TokenRepository interface {
	GetToken(ctx context.Context, id string) (*Token, error)
	SetToken(ctx context.Context, id string) (*Token, error)
	DeleteToken(ctx context.Context, id string) error
}
