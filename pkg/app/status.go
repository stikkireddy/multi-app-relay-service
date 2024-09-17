package app

// Status represents the state of an app.
type Status string

const (
	StatusSetup      Status = "setup"
	StatusStarting   Status = "starting"
	StatusRunning    Status = "running"
	StatusTerminated Status = "terminated"
)

// IsValid checks if a given status is valid.
func (s Status) IsValid() bool {
	switch s {
	case StatusStarting, StatusSetup, StatusRunning, StatusTerminated:
		return true
	}
	return false
}

// String returns the string representation of the status.
func (s Status) String() string {
	return string(s)
}
