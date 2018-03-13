package db

import "context"

type MockUserEmails struct {
	GetPrimaryEmail func(ctx context.Context, id int32) (email string, verified bool, err error)
	ListByUser      func(ctx context.Context, id int32) ([]*UserEmail, error)
}
