package model

// EmploymentType classifies a staff account (full-time vs part-time). Generic
// HR attribute — kept on User for projects that distinguish employment status.
type EmploymentType string

const (
	EmploymentFulltime EmploymentType = "fulltime"
	EmploymentParttime EmploymentType = "parttime"
)

// Valid reports whether e is a known employment type.
func (e EmploymentType) Valid() bool {
	switch e {
	case EmploymentFulltime, EmploymentParttime:
		return true
	}
	return false
}
