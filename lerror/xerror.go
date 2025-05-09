package lerror

import (
	"errors"
	"fmt"
	"reflect"
)

type (
	XError struct {
		Status  int //Http status code
		Code    int //Code in response body
		Message string
	}
)

func (e *XError) Error() string {
	return fmt.Sprintf("Status code: %d, Error code: %d, Message: %s", e.Status, e.Code, e.Message)
}

func IsLError(err error) bool {
	e := &XError{}
	return errors.As(err, &e)
}

func Unwrap(err error) *XError {
	stt := &XError{}
	for {
		if err != nil && reflect.TypeOf(err) != reflect.TypeOf(stt) {
			err = errors.Unwrap(err)
		} else {
			break
		}
	}

	ps, ok := err.(*XError)
	if !ok {
		return nil
	}
	return ps
}
