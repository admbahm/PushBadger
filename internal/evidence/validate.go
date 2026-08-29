// Package evidence implements the deterministic Evidence Contract v1 trust
// boundary: strict JSON parsing, structural validation, and semantic admission.
package evidence

// Validate admits one schema_version 3 Evidence Contract document or returns
// a deterministic error identifying the first rejected condition.
func Validate(data []byte) error {
	value, err := parse(data)
	if err != nil {
		return err
	}
	if err := validateStructure(value); err != nil {
		return err
	}
	return validatePolicy(value.(object))
}
