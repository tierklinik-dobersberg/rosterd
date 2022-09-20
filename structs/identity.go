package structs

type (
	// User describes the user object.
	User struct {
		Name     string   `json:"name" hcl:",label"`
		Roles    []string `json:"roles" hcl:"roles,optional"`
		Disabled *bool    `json:"disabled,omitempty" hcl:"disabled"`
	}
)
