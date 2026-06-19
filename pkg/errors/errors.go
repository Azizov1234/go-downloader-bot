package errors

import stderrors "errors"

var (
	ErrUnsupportedPlatform = stderrors.New("unsupported platform")
	ErrInvalidURL          = stderrors.New("invalid url")
	ErrCacheMiss           = stderrors.New("cache miss")
	ErrOversized           = stderrors.New("media is oversized")
	ErrForbidden           = stderrors.New("forbidden")
)
