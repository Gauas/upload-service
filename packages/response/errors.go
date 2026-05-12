package response

import "net/http"

type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }

func NewError(code int, msg string) error {
	return &Error{Code: code, Message: msg}
}

var (
	ErrorNotFound     = &Error{Code: http.StatusNotFound, Message: "resource not found"}
	ErrorForbidden    = &Error{Code: http.StatusForbidden, Message: "forbidden"}
	ErrorUnauthorized = &Error{Code: http.StatusUnauthorized, Message: "unauthorized"}
)
