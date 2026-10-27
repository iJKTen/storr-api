package pipeline

import "errors"

var (
	// ErrInvalidProjectType is returned when a theme-only operation is attempted on a non-theme store.
	ErrInvalidProjectType = errors.New("invalid project type for this operation")
	// ErrThemeNotRunning is returned when a theme interaction is attempted with no active theme task.
	ErrThemeNotRunning = errors.New("no running theme task")
)
