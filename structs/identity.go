package structs

import "time"

type (
	// User describes the user object.
	User struct {
		Name            string        `json:"name"`
		Roles           []string      `json:"roles"`
		Disabled        *bool         `json:"disabled,omitempty"`
		WorktimePerWeek time.Duration `json:"worktimePerWeek"`
	}
)
