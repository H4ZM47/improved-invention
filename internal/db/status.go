package db

const (
	StatusBacklog   = "backlog"
	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusBlocked   = "blocked"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

var validStatuses = map[string]struct{}{
	StatusBacklog:   {},
	StatusActive:    {},
	StatusPaused:    {},
	StatusBlocked:   {},
	StatusCompleted: {},
	StatusCancelled: {},
}

// IsValidStatus reports whether value is one of the supported lifecycle statuses.
func IsValidStatus(value string) bool {
	_, ok := validStatuses[value]
	return ok
}

// ValidStatuses returns the supported lifecycle statuses in display order.
func ValidStatuses() []string {
	return []string{
		StatusBacklog,
		StatusActive,
		StatusPaused,
		StatusBlocked,
		StatusCompleted,
		StatusCancelled,
	}
}
