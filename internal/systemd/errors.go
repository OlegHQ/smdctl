package systemd

import "errors"

var (
	// ErrServiceNotFound is returned when a service doesn't exist
	ErrServiceNotFound = errors.New("service not found")

	// ErrServiceAlreadyExists is returned when trying to create a service that already exists
	ErrServiceAlreadyExists = errors.New("service already exists")

	// ErrInvalidServiceName is returned when service name is invalid
	ErrInvalidServiceName = errors.New("invalid service name: must contain only alphanumeric characters, hyphens, and underscores")

	// ErrServiceFailed is returned when a service fails to start
	ErrServiceFailed = errors.New("service failed to start")
)
