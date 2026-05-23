// Package tools provides MCP tool handlers for the Godville API.
package tools

import "github.com/cockroachdb/errors"

// ErrValidation indicates invalid parameters provided by the caller.
var ErrValidation = errors.New("validation error")

// ErrAPI indicates a failure communicating with the Godville API.
var ErrAPI = errors.New("godville request error")

// ValidationErr marks an error as a validation error. Re-exported so future
// tools with parameters can flag bad input consistently.
func ValidationErr(err error) error {
	//nolint:wrapcheck // Mark adds a sentinel category, the caller already provides context.
	return errors.Mark(err, ErrValidation)
}

func apiErr(msg string, err error) error {
	//nolint:wrapcheck // Mark adds a sentinel category on top of Wrap which provides context.
	return errors.Mark(errors.Wrap(err, msg), ErrAPI)
}
