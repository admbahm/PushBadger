package evidence

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
)

var (
	verificationIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\n?\z`)
	findingIDPattern      = regexp.MustCompile(`^DI-REV-[0-9]+\n?\z`)
)

// validateStructure is a schema-specific Go implementation of
// .review/review-evidence.schema.json. The artifact fingerprint in the tests
// makes schema drift explicit instead of silently broadening this boundary.
func validateStructure(value any) error {
	root, err := requireObject(value, "$")
	if err != nil {
		return err
	}
	if err := requireKeys(root, "$", "schema_version", "review", "verification", "findings"); err != nil {
		return err
	}

	version, err := requireString(root["schema_version"], `$["schema_version"]`, 0)
	if err != nil {
		return err
	}
	if version != "3" {
		return schemaError(`$["schema_version"]`, `must equal "3"`)
	}
	if err := validateReview(root["review"]); err != nil {
		return err
	}
	if err := validateVerification(root["verification"]); err != nil {
		return err
	}
	return validateFindings(root["findings"])
}

func validateReview(value any) error {
	path := `$["review"]`
	review, err := requireObject(value, path)
	if err != nil {
		return err
	}
	if err := requireKeys(review, path, "repository", "pr", "base_sha", "head_sha", "verdict"); err != nil {
		return err
	}
	if _, err := requireString(review["repository"], propertyPath(path, "repository"), 1); err != nil {
		return err
	}
	if err := requirePositiveInteger(review["pr"], propertyPath(path, "pr")); err != nil {
		return err
	}
	if _, err := requireString(review["base_sha"], propertyPath(path, "base_sha"), 7); err != nil {
		return err
	}
	if _, err := requireString(review["head_sha"], propertyPath(path, "head_sha"), 7); err != nil {
		return err
	}
	return requireEnum(review["verdict"], propertyPath(path, "verdict"), "PASS", "FAIL", "INCONCLUSIVE")
}

func validateVerification(value any) error {
	path := `$["verification"]`
	records, err := requireArray(value, path, 0)
	if err != nil {
		return err
	}
	for index, value := range records {
		recordPath := fmt.Sprintf("%s[%d]", path, index)
		record, err := requireObject(value, recordPath)
		if err != nil {
			return err
		}
		if err := requireKeys(record, recordPath, "id", "type", "required", "outcome", "detail"); err != nil {
			return err
		}
		if err := rejectAdditional(record, recordPath, "id", "type", "required", "outcome", "detail", "command"); err != nil {
			return err
		}
		id, err := requireString(record["id"], propertyPath(recordPath, "id"), 0)
		if err != nil {
			return err
		}
		if !verificationIDPattern.MatchString(id) {
			return schemaError(propertyPath(recordPath, "id"), "does not match the required pattern")
		}
		if err := requireEnum(record["type"], propertyPath(recordPath, "type"), "test", "command", "inspection", "experiment"); err != nil {
			return err
		}
		if _, ok := record["required"].(bool); !ok {
			return schemaError(propertyPath(recordPath, "required"), "must be a boolean")
		}
		if err := requireEnum(record["outcome"], propertyPath(recordPath, "outcome"), "PASS", "FAIL", "NOT_RUN", "UNKNOWN"); err != nil {
			return err
		}
		if _, err := requireString(record["detail"], propertyPath(recordPath, "detail"), 1); err != nil {
			return err
		}
		if command, exists := record["command"]; exists {
			if _, err := requireString(command, propertyPath(recordPath, "command"), 1); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFindings(value any) error {
	path := `$["findings"]`
	findings, err := requireArray(value, path, 0)
	if err != nil {
		return err
	}
	for index, value := range findings {
		findingPath := fmt.Sprintf("%s[%d]", path, index)
		finding, err := requireObject(value, findingPath)
		if err != nil {
			return err
		}
		if err := requireKeys(finding, findingPath,
			"id", "classification", "severity", "claim", "affected_behavior", "locations", "evidence",
			"expected", "observed", "reproduction", "attempted_disproof", "remaining_uncertainty"); err != nil {
			return err
		}

		id, err := requireString(finding["id"], propertyPath(findingPath, "id"), 0)
		if err != nil {
			return err
		}
		if !findingIDPattern.MatchString(id) {
			return schemaError(propertyPath(findingPath, "id"), "does not match the required pattern")
		}
		if err := requireEnum(finding["classification"], propertyPath(findingPath, "classification"), "PROVEN", "HIGH_CONFIDENCE", "POSSIBLE", "STYLE"); err != nil {
			return err
		}
		if err := requireEnum(finding["severity"], propertyPath(findingPath, "severity"), "critical", "high", "medium", "low"); err != nil {
			return err
		}
		for _, name := range []string{"claim", "affected_behavior", "expected", "observed"} {
			if _, err := requireString(finding[name], propertyPath(findingPath, name), 1); err != nil {
				return err
			}
		}
		if _, err := requireString(finding["remaining_uncertainty"], propertyPath(findingPath, "remaining_uncertainty"), 0); err != nil {
			return err
		}
		if err := validateLocations(finding["locations"], propertyPath(findingPath, "locations")); err != nil {
			return err
		}
		if err := validateFindingEvidence(finding["evidence"], propertyPath(findingPath, "evidence")); err != nil {
			return err
		}
		if err := validateStringArray(finding["reproduction"], propertyPath(findingPath, "reproduction")); err != nil {
			return err
		}
		if err := validateStringArray(finding["attempted_disproof"], propertyPath(findingPath, "attempted_disproof")); err != nil {
			return err
		}
	}
	return nil
}

func validateLocations(value any, path string) error {
	locations, err := requireArray(value, path, 1)
	if err != nil {
		return err
	}
	for index, value := range locations {
		locationPath := fmt.Sprintf("%s[%d]", path, index)
		location, err := requireObject(value, locationPath)
		if err != nil {
			return err
		}
		if err := requireKeys(location, locationPath, "path"); err != nil {
			return err
		}
		if err := rejectAdditional(location, locationPath, "path", "symbol"); err != nil {
			return err
		}
		if _, err := requireString(location["path"], propertyPath(locationPath, "path"), 1); err != nil {
			return err
		}
		if symbol, exists := location["symbol"]; exists {
			if _, err := requireString(symbol, propertyPath(locationPath, "symbol"), 0); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFindingEvidence(value any, path string) error {
	records, err := requireArray(value, path, 1)
	if err != nil {
		return err
	}
	for index, value := range records {
		recordPath := fmt.Sprintf("%s[%d]", path, index)
		record, err := requireObject(value, recordPath)
		if err != nil {
			return err
		}
		if err := requireKeys(record, recordPath, "type", "detail"); err != nil {
			return err
		}
		if err := requireEnum(record["type"], propertyPath(recordPath, "type"), "test", "command", "code_path", "history", "experiment"); err != nil {
			return err
		}
		if _, err := requireString(record["detail"], propertyPath(recordPath, "detail"), 1); err != nil {
			return err
		}
		if qualification, exists := record["qualification"]; exists {
			if err := requireEnum(qualification, propertyPath(recordPath, "qualification"), "OBSERVED", "STRONG", "UNAVOIDABLE"); err != nil {
				return err
			}
		}
		for _, name := range []string{"base", "head"} {
			if outcome, exists := record[name]; exists {
				if err := requireEnum(outcome, propertyPath(recordPath, name), "PASS", "FAIL", "NOT_RUN", "UNKNOWN"); err != nil {
					return err
				}
			}
		}

		typeName := record["type"].(string)
		_, hasQualification := record["qualification"]
		if typeName == "code_path" && !hasQualification {
			return schemaError(recordPath, `missing required property "qualification" for code_path evidence`)
		}
		if typeName != "code_path" && hasQualification {
			return schemaError(propertyPath(recordPath, "qualification"), "is only allowed for code_path evidence")
		}
	}
	return nil
}

func validateStringArray(value any, path string) error {
	values, err := requireArray(value, path, 0)
	if err != nil {
		return err
	}
	for index, value := range values {
		if _, err := requireString(value, fmt.Sprintf("%s[%d]", path, index), 1); err != nil {
			return err
		}
	}
	return nil
}

func requireObject(value any, path string) (object, error) {
	result, ok := value.(object)
	if !ok {
		return nil, schemaError(path, "must be an object")
	}
	return result, nil
}

func requireArray(value any, path string, minItems int) (array, error) {
	result, ok := value.(array)
	if !ok {
		return nil, schemaError(path, "must be an array")
	}
	if len(result) < minItems {
		return nil, schemaError(path, fmt.Sprintf("must contain at least %d item(s)", minItems))
	}
	return result, nil
}

func requireString(value any, path string, minLength int) (string, error) {
	result, ok := value.(string)
	if !ok {
		return "", schemaError(path, "must be a string")
	}
	if decodedCharacterCount(result) < minLength {
		return "", schemaError(path, fmt.Sprintf("must contain at least %d character(s)", minLength))
	}
	return result, nil
}

func requireEnum(value any, path string, allowed ...string) error {
	text, ok := value.(string)
	if !ok {
		return schemaError(path, "must be a string")
	}
	for _, candidate := range allowed {
		if text == candidate {
			return nil
		}
	}
	return schemaError(path, fmt.Sprintf("must be one of %s", strings.Join(allowed, ", ")))
}

func requirePositiveInteger(value any, path string) error {
	n, ok := value.(number)
	if !ok {
		return schemaError(path, "must be an integer")
	}
	integer, positive := representedInteger(n.raw)
	if !integer {
		return schemaError(path, fmt.Sprintf("number %q must be an integer", n.raw))
	}
	if !positive {
		return schemaError(path, "must be at least 1")
	}
	return nil
}

// representedInteger reports whether a syntactically valid JSON number is
// mathematically integral and, if so, whether it is greater than zero. It
// examines decimal scale exactly and imposes no lexical digit ceiling.
func representedInteger(raw string) (integral bool, positive bool) {
	negative := strings.HasPrefix(raw, "-")
	unsigned := strings.TrimPrefix(raw, "-")

	mantissa := unsigned
	exponentText := "0"
	if index := strings.IndexAny(unsigned, "eE"); index >= 0 {
		mantissa = unsigned[:index]
		exponentText = unsigned[index+1:]
	}

	integerPart := mantissa
	fractionPart := ""
	if index := strings.IndexByte(mantissa, '.'); index >= 0 {
		integerPart = mantissa[:index]
		fractionPart = mantissa[index+1:]
	}
	digits := integerPart + fractionPart
	nonzero := strings.Trim(digits, "0") != ""
	if !nonzero {
		return true, false
	}

	exponent := new(big.Int)
	if _, ok := exponent.SetString(exponentText, 10); !ok {
		return false, false
	}
	fractionDigits := big.NewInt(int64(len(fractionPart)))
	scale := new(big.Int).Sub(fractionDigits, exponent)
	if scale.Sign() <= 0 {
		return true, !negative
	}

	trailingZeros := 0
	for index := len(digits) - 1; index >= 0 && digits[index] == '0'; index-- {
		trailingZeros++
	}
	if scale.Cmp(big.NewInt(int64(trailingZeros))) > 0 {
		return false, false
	}
	return true, !negative
}

func requireKeys(value object, path string, keys ...string) error {
	for _, key := range keys {
		if _, exists := value[key]; !exists {
			return schemaError(path, fmt.Sprintf("missing required property %q", key))
		}
	}
	return nil
}

func rejectAdditional(value object, path string, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	var extras []string
	for key := range value {
		if _, ok := allowedSet[key]; !ok {
			extras = append(extras, key)
		}
	}
	if len(extras) == 0 {
		return nil
	}
	sort.Strings(extras)
	return schemaError(path, fmt.Sprintf("additional property %q is not allowed", extras[0]))
}

func schemaError(path, message string) error {
	return fmt.Errorf("schema error at %s: %s", path, message)
}
