package bot

import "errors"

var (
	ErrUnknownEvent       = errors.New("unknown event")
	ErrDialogNotFound     = errors.New("dialog not found")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUnsupportedFeature = errors.New("adapter feature unsupported")
)
