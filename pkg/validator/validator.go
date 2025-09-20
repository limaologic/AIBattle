// Package validator provides answer validation functionality for the Reverse Challenge System.
// Supports multiple validation types including exact matching, numeric tolerance, and regex patterns.
// Used by challengers to verify solver responses against expected answers.
package validator

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"reverse-challenge-system/pkg/models"
)

// Validator provides methods for validating solver answers against challenge solutions.
// Supports different validation strategies based on the challenge requirements.
type Validator struct{}

// NewValidator creates a new validator instance.
// Returns a validator ready to process validation rules.
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateAnswer validates a solver's answer against the specified validation rule.
// Routes to the appropriate validation method based on the rule type.
// Returns true if the answer is valid, false otherwise, along with any validation errors.
func (v *Validator) ValidateAnswer(rule models.ValidationRule, receivedAnswer string) (bool, error) {
	switch rule.Type {
	case "ExactMatch":
		return v.validateExactMatch(rule, receivedAnswer)
	case "NumericTolerance":
		return v.validateNumericTolerance(rule, receivedAnswer)
	case "Regex":
		return v.validateRegex(rule, receivedAnswer)
	default:
		return false, fmt.Errorf("unknown validation rule type: %s", rule.Type)
	}
}

// validateExactMatch performs exact string matching validation with optional case sensitivity.
// Compares the received answer directly with the expected answer.
// Defaults to case-sensitive comparison if no parameters are provided.
func (v *Validator) validateExactMatch(rule models.ValidationRule, receivedAnswer string) (bool, error) {
	var params models.ExactMatchParams

	// Default to case sensitive if no params
	params.CaseSensitive = true

	if rule.Params != nil {
		if err := json.Unmarshal(rule.Params, &params); err != nil {
			return false, fmt.Errorf("failed to unmarshal ExactMatch params: %w", err)
		}
	}

	if params.CaseSensitive {
		return rule.Answer == receivedAnswer, nil
	}

	return strings.EqualFold(rule.Answer, receivedAnswer), nil
}

// validateNumericTolerance performs numeric validation with a specified tolerance.
// Parses both expected and received answers as floating-point numbers and checks
// if the absolute difference is within the allowed tolerance.
func (v *Validator) validateNumericTolerance(rule models.ValidationRule, receivedAnswer string) (bool, error) {
	var params models.NumericToleranceParams

	if rule.Params == nil {
		return false, fmt.Errorf("NumericTolerance requires params")
	}

	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return false, fmt.Errorf("failed to unmarshal NumericTolerance params: %w", err)
	}

	expectedValue, err := strconv.ParseFloat(rule.Answer, 64)
	if err != nil {
		return false, fmt.Errorf("failed to parse expected answer as float: %w", err)
	}

	receivedValue, err := strconv.ParseFloat(receivedAnswer, 64)
	if err != nil {
		return false, fmt.Errorf("failed to parse received answer as float: %w", err)
	}

	diff := math.Abs(expectedValue - receivedValue)
	return diff <= params.Tolerance, nil
}

// validateRegex performs pattern matching validation using regular expressions.
// Compiles the regex pattern from parameters and tests it against the received answer.
// Useful for flexible text validation where exact matches are not required.
func (v *Validator) validateRegex(rule models.ValidationRule, receivedAnswer string) (bool, error) {
	var params models.RegexParams

	if rule.Params == nil {
		return false, fmt.Errorf("Regex validation requires params")
	}

	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return false, fmt.Errorf("failed to unmarshal Regex params: %w", err)
	}

	regex, err := regexp.Compile(params.Pattern)
	if err != nil {
		return false, fmt.Errorf("failed to compile regex pattern: %w", err)
	}

	return regex.MatchString(receivedAnswer), nil
}

// Helper functions to create validation rules

// CreateExactMatchRule creates a validation rule for exact string matching.
// Sets up the rule with the expected answer and case sensitivity preference.
// Commonly used for text-based challenges where precise answers are required.
func CreateExactMatchRule(answer string, caseSensitive bool) models.ValidationRule {
	params := models.ExactMatchParams{CaseSensitive: caseSensitive}
	paramsJSON, _ := json.Marshal(params)

	return models.ValidationRule{
		Type:   "ExactMatch",
		Params: paramsJSON,
		Answer: answer,
	}
}

// CreateNumericToleranceRule creates a validation rule for numeric answers with tolerance.
// Allows for small differences between expected and received numeric values.
// Ideal for mathematical challenges where floating-point precision may vary.
func CreateNumericToleranceRule(answer string, tolerance float64) models.ValidationRule {
	params := models.NumericToleranceParams{Tolerance: tolerance}
	paramsJSON, _ := json.Marshal(params)

	return models.ValidationRule{
		Type:   "NumericTolerance",
		Params: paramsJSON,
		Answer: answer,
	}
}

// CreateRegexRule creates a validation rule for pattern-based matching.
// Uses regular expressions to validate answers against flexible patterns.
// Useful for challenges where multiple valid answer formats are acceptable.
func CreateRegexRule(pattern string) models.ValidationRule {
	params := models.RegexParams{Pattern: pattern}
	paramsJSON, _ := json.Marshal(params)

	return models.ValidationRule{
		Type:   "Regex",
		Params: paramsJSON,
		Answer: "", // For regex, we don't store a specific answer
	}
}
