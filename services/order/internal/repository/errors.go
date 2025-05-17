package repository

import "errors"

var (
	// ErrOrderNotFound is returned when an order is not found
	ErrOrderNotFound = errors.New("order not found")
	
	// ErrOrderLocationNotFound is returned when an order location is not found
	ErrOrderLocationNotFound = errors.New("order location not found")
	
	// ErrDuplicateOrder is returned when attempting to create an order with an ID that already exists
	ErrDuplicateOrder = errors.New("duplicate order")
) 