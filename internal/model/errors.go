package model

type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string {
	return e.Msg
}

type NotFoundError struct {
	Msg string
}

func (e *NotFoundError) Error() string {
	return e.Msg
}

type ConflictError struct {
	Msg string
}

func (e *ConflictError) Error() string {
	return e.Msg
}
