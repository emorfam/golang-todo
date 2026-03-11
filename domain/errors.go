package domain

import "errors"

// ErrNotFound is returned by the repository when a requested resource does not exist.
// It is a domain-level sentinel so that repository/ does not need to import internal/apierror.
var ErrNotFound = errors.New("not found")
