package app

// Type represents the state of an app.
type Type string

const (
	TypePython Type = "python"
	TypeR      Type = "r"
	TypeNodejs Type = "nodejs"
)

// IsValid checks if a given status is valid.
func (s Type) IsValid() bool {
	switch s {
	case TypeNodejs, TypeR, TypePython:
		return true
	}
	return false
}

// String returns the string representation of the status.
func (s Type) String() string {
	return string(s)
}
