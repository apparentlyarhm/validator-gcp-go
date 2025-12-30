package apperror

import (
	"errors"

	"google.golang.org/api/googleapi"
)

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

	// Will help us for all socket/TCP connection failures
	ErrInternal = errors.New("")

	INTERNAL_MESSAGE = "Internal Server Error"
)

// might expand it later.
type ErrorResponse struct {
	Code    int16  `json:"code"`
	Message string `json:"message"`
}

/*
translates a raw GCP error into a domain-specific apperror.
If it's not a known mapping, it returns the original error (which will result in a 500).
*/
func MapError(err error) error {
	// at the time of writing this im not sure what the best practices here are. this is logically the same thing as what i did in my java
	// app.

	if err == nil {
		return nil
	}

	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		switch gErr.Code {
		case 404:
			return ErrNotFound
		case 403:
			return ErrForbidden
		case 409:
			return ErrConflict
		case 400:
			return ErrBadRequest
		}
	}

	// If it's not a specific code we care about (like 503 Unavailable, 500 Internal),
	// we return the original error. The Handler will log it and return HTTP 500.
	return err
}
