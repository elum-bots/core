package gemini

import "errors"

var (
	ErrGenerationRejected    = errors.New("generation rejected")
	ErrGenerationUnavailable = errors.New("generation unavailable")
	ErrGenerationRateLimited = errors.New("generation rate limited")
)

type RateLimitError struct {
	Err error
}

func (e RateLimitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ErrGenerationRateLimited.Error()
}

func (e RateLimitError) Unwrap() error {
	return e.Err
}
