package apperror

import "errors"

var (

	// This is based on the exceptions I defined in my original spring boot app.

	// ErrNotFound: Matches NotFoundException (Custom & GCP)
	ErrNotFound = errors.New("the queried resource was not found")

	// ErrConflict: Matches AlreadyExistsException
	ErrConflict = errors.New("resource already exists")

	// ErrBadRequest: Matches BadRequestException, MissingFormatArgument, AssertionError, HttpMessageNotReadable
	ErrBadRequest = errors.New("check request inputs")

	// ErrForbidden: Matches ForbiddenException and GCP PERMISSION_DENIED
	ErrForbidden = errors.New("you do not have permission to perform this action")
)

// might expand it later.
type ErrorResponse struct {
	Code    int16  `json:"code"`
	Message string `json:"message"`
}
