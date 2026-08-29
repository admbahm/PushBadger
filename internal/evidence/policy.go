package evidence

import (
	"fmt"
	"unicode"
)

func validatePolicy(root object) error {
	verification := root["verification"].(array)
	findings := root["findings"].(array)

	if err := uniqueIDs(verification, `$["verification"]`); err != nil {
		return err
	}
	if err := uniqueIDs(findings, `$["findings"]`); err != nil {
		return err
	}

	actionableCount := 0
	for index, value := range findings {
		finding := value.(object)
		classification := finding["classification"].(string)
		if classification != "PROVEN" && classification != "HIGH_CONFIDENCE" {
			continue
		}
		actionableCount++
		if err := validateActionableFinding(finding, index); err != nil {
			return err
		}
	}

	requiredCount := 0
	allRequiredPassed := true
	incompleteRequired := false
	for _, value := range verification {
		record := value.(object)
		if !record["required"].(bool) {
			continue
		}
		requiredCount++
		outcome := record["outcome"].(string)
		if outcome != "PASS" {
			allRequiredPassed = false
		}
		if outcome == "NOT_RUN" || outcome == "UNKNOWN" {
			incompleteRequired = true
		}
	}

	verdict := root["review"].(object)["verdict"].(string)
	switch verdict {
	case "PASS":
		if requiredCount == 0 {
			return policyError(`$["review"]["verdict"]`, "PASS requires at least one required verification")
		}
		if !allRequiredPassed {
			return policyError(`$["review"]["verdict"]`, "PASS requires every required verification to PASS")
		}
		if actionableCount != 0 {
			return policyError(`$["review"]["verdict"]`, "PASS cannot contain actionable findings")
		}
	case "FAIL":
		if actionableCount == 0 {
			return policyError(`$["review"]["verdict"]`, "FAIL requires at least one admissible actionable finding")
		}
	case "INCONCLUSIVE":
		if !incompleteRequired {
			return policyError(`$["review"]["verdict"]`, "INCONCLUSIVE requires a required verification with outcome NOT_RUN or UNKNOWN")
		}
		if actionableCount != 0 {
			return policyError(`$["review"]["verdict"]`, "INCONCLUSIVE cannot contain actionable findings")
		}
	}
	return validateSemanticStrings(root)
}

func uniqueIDs(records array, path string) error {
	seen := make(map[string]struct{}, len(records))
	for index, value := range records {
		id := value.(object)["id"].(string)
		if _, exists := seen[id]; exists {
			return policyError(fmt.Sprintf("%s[%d][\"id\"]", path, index), fmt.Sprintf("duplicate id %q", id))
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateActionableFinding(finding object, index int) error {
	path := fmt.Sprintf(`$["findings"][%d]`, index)
	reproduction := finding["reproduction"].(array)
	if len(reproduction) == 0 {
		return policyError(propertyPath(path, "reproduction"), "actionable findings require reproduction steps or an executable reasoning path")
	}
	attempts := finding["attempted_disproof"].(array)
	if len(attempts) == 0 {
		return policyError(propertyPath(path, "attempted_disproof"), "actionable findings require at least one attempted disproof")
	}

	evidence := finding["evidence"].(array)
	hasStrongCodePath := false
	hasUnavoidableCodePath := false
	for _, value := range evidence {
		record := value.(object)
		typeName := record["type"].(string)
		if typeName == "code_path" {
			qualification := record["qualification"].(string)
			if qualification == "STRONG" || qualification == "UNAVOIDABLE" {
				hasStrongCodePath = true
			}
			if qualification == "UNAVOIDABLE" {
				hasUnavoidableCodePath = true
			}
		}
	}

	classification := finding["classification"].(string)
	switch classification {
	case "HIGH_CONFIDENCE":
		if !hasStrongCodePath {
			return policyError(propertyPath(path, "evidence"), "HIGH_CONFIDENCE requires a STRONG or UNAVOIDABLE code path")
		}
		return nil
	case "PROVEN":
		hasCompletedExecutable := false
		for evidenceIndex, value := range evidence {
			record := value.(object)
			typeName := record["type"].(string)
			if typeName != "test" && typeName != "command" && typeName != "experiment" {
				continue
			}
			if !hasCompletedOutcome(record) {
				return policyError(
					fmt.Sprintf(`%s["evidence"][%d]`, path, evidenceIndex),
					"PROVEN executable evidence must include at least one structured PASS or FAIL outcome",
				)
			}
			hasCompletedExecutable = true
		}
		if !hasCompletedExecutable && !hasUnavoidableCodePath {
			return policyError(propertyPath(path, "evidence"), "PROVEN findings require executed evidence or an unavoidable code path")
		}
	}
	return nil
}

func hasCompletedOutcome(record object) bool {
	for _, name := range []string{"base", "head"} {
		if outcome, exists := record[name].(string); exists && (outcome == "PASS" || outcome == "FAIL") {
			return true
		}
	}
	return false
}

func validateSemanticStrings(root object) error {
	review := root["review"].(object)
	for _, name := range []string{"repository", "base_sha", "head_sha"} {
		if err := requireNonblank(review[name].(string), propertyPath(`$["review"]`, name)); err != nil {
			return err
		}
	}

	for verificationIndex, value := range root["verification"].(array) {
		record := value.(object)
		path := fmt.Sprintf(`$["verification"][%d]`, verificationIndex)
		for _, name := range []string{"id", "detail"} {
			if err := requireNonblank(record[name].(string), propertyPath(path, name)); err != nil {
				return err
			}
		}
		if command, exists := record["command"].(string); exists {
			if err := requireNonblank(command, propertyPath(path, "command")); err != nil {
				return err
			}
		}
	}

	for findingIndex, value := range root["findings"].(array) {
		finding := value.(object)
		path := fmt.Sprintf(`$["findings"][%d]`, findingIndex)
		for _, name := range []string{"claim", "affected_behavior", "expected", "observed", "remaining_uncertainty"} {
			if err := requireNonblank(finding[name].(string), propertyPath(path, name)); err != nil {
				return err
			}
		}

		for locationIndex, value := range finding["locations"].(array) {
			location := value.(object)
			locationPath := fmt.Sprintf(`%s["locations"][%d]`, path, locationIndex)
			if err := requireNonblank(location["path"].(string), propertyPath(locationPath, "path")); err != nil {
				return err
			}
			if symbol, exists := location["symbol"].(string); exists {
				if err := requireNonblank(symbol, propertyPath(locationPath, "symbol")); err != nil {
					return err
				}
			}
		}

		for evidenceIndex, value := range finding["evidence"].(array) {
			record := value.(object)
			evidencePath := fmt.Sprintf(`%s["evidence"][%d]["detail"]`, path, evidenceIndex)
			if err := requireNonblank(record["detail"].(string), evidencePath); err != nil {
				return err
			}
		}

		for _, name := range []string{"reproduction", "attempted_disproof"} {
			for recordIndex, value := range finding[name].(array) {
				recordPath := fmt.Sprintf(`%s["%s"][%d]`, path, name, recordIndex)
				if err := requireNonblank(value.(string), recordPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func requireNonblank(value, path string) error {
	for _, character := range value {
		if !isReferenceWhitespace(character) {
			return nil
		}
	}
	return policyError(path, "must contain non-whitespace text")
}

// Python str.strip includes the Unicode White_Space property plus these four
// information separators. Mirror that accepted-validator behavior exactly.
func isReferenceWhitespace(character rune) bool {
	return unicode.IsSpace(character) || (character >= '\u001c' && character <= '\u001f')
}

func policyError(path, message string) error {
	return fmt.Errorf("policy error at %s: %s", path, message)
}
