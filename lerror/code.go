package lerror

import "net/http"

type LCode int

const (
	PermissionDenied LCode = 4300
	Unauthorized     LCode = 4100
	UserInactive     LCode = 4101
	InvalidToken     LCode = 4102
	InvalidData      LCode = 4000
	InvalidJson      LCode = 4001
	InternalServer   LCode = 5000
)

var errorMap = map[LCode]*XError{
	InvalidData: {
		Status:  http.StatusBadRequest,
		Message: "Invalid data",
	},
	Unauthorized: {
		Status:  http.StatusUnauthorized,
		Message: "Unauthorized",
	},
	InvalidJson: {
		Status:  http.StatusBadRequest,
		Message: "Invalid Json data",
	},
	UserInactive: {
		Status:  http.StatusUnauthorized,
		Message: "Inactive account",
	},
	InvalidToken: {
		Status:  http.StatusUnauthorized,
		Message: "Session expired",
	},
	PermissionDenied: {
		Status:  http.StatusForbidden,
		Message: "You do not have permission",
	},
	InternalServer: {
		Status:  http.StatusInternalServerError,
		Message: "Internal Server",
	},
}

func (c LCode) ToInt() int {
	return int(c)
}

func (c LCode) ToError(message ...string) *XError {
	r := errorMap[c]
	err := &XError{
		Status:  r.Status,
		Code:    c.ToInt(),
		Message: r.Message,
	}
	if len(message) != 0 {
		err.Message = message[0]
	}
	return err
}
