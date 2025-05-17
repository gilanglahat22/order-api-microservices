package repository

import "errors"

var (
	// ErrProviderNotFound is returned when a provider is not found
	ErrProviderNotFound = errors.New("provider not found")
	
	// ErrDuplicateProvider is returned when attempting to create a provider with an ID that already exists
	ErrDuplicateProvider = errors.New("duplicate provider")
) 