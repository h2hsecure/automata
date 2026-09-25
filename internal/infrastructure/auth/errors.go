package auth

import "errors"

var (
	// ErrRefreshTokenExpired indicates the refresh token has passed its expiration time.
	ErrRefreshTokenExpired = errors.New("refresh token expired")
	// ErrRefreshTokenInvalid indicates the refresh token is malformed, missing claims,
	// or has an invalid signature.
	ErrRefreshTokenInvalid = errors.New("invalid refresh token")
)
