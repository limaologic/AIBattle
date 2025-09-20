package validator

import (
	"encoding/json"
	"testing"

	"reverse-challenge-system/pkg/models"
)

func TestValidator_ValidateExactMatch(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name           string
		answer         string
		caseSensitive  bool
		receivedAnswer string
		expectedValid  bool
		expectError    bool
	}{
		{
			name:           "CaseSensitive_ExactMatch",
			answer:         "Hello World",
			caseSensitive:  true,
			receivedAnswer: "Hello World",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "CaseSensitive_Mismatch",
			answer:         "Hello World",
			caseSensitive:  true,
			receivedAnswer: "hello world",
			expectedValid:  false,
			expectError:    false,
		},
		{
			name:           "CaseInsensitive_Match",
			answer:         "Hello World",
			caseSensitive:  false,
			receivedAnswer: "hello world",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "CaseInsensitive_Mismatch",
			answer:         "Hello World",
			caseSensitive:  false,
			receivedAnswer: "Goodbye World",
			expectedValid:  false,
			expectError:    false,
		},
		{
			name:           "EmptyStrings_Match",
			answer:         "",
			caseSensitive:  true,
			receivedAnswer: "",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "SpecialCharacters_Match",
			answer:         "Test!@#$%^&*()",
			caseSensitive:  true,
			receivedAnswer: "Test!@#$%^&*()",
			expectedValid:  true,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := CreateExactMatchRule(tt.answer, tt.caseSensitive)

			isValid, err := validator.ValidateAnswer(rule, tt.receivedAnswer)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if isValid != tt.expectedValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectedValid, isValid)
			}
		})
	}
}

func TestValidator_ValidateNumericTolerance(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name           string
		answer         string
		tolerance      float64
		receivedAnswer string
		expectedValid  bool
		expectError    bool
	}{
		{
			name:           "ExactMatch",
			answer:         "42.0",
			tolerance:      0.01,
			receivedAnswer: "42.0",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "WithinTolerance_Lower",
			answer:         "42.0",
			tolerance:      0.1,
			receivedAnswer: "41.95",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "WithinTolerance_Upper",
			answer:         "42.0",
			tolerance:      0.1,
			receivedAnswer: "42.05",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "OutsideTolerance",
			answer:         "42.0",
			tolerance:      0.01,
			receivedAnswer: "42.1",
			expectedValid:  false,
			expectError:    false,
		},
		{
			name:           "NegativeNumbers",
			answer:         "-10.5",
			tolerance:      0.1,
			receivedAnswer: "-10.45",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "ZeroTolerance",
			answer:         "100",
			tolerance:      0.0,
			receivedAnswer: "100.0",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "InvalidExpectedAnswer",
			answer:         "not_a_number",
			tolerance:      0.1,
			receivedAnswer: "42.0",
			expectedValid:  false,
			expectError:    true,
		},
		{
			name:           "InvalidReceivedAnswer",
			answer:         "42.0",
			tolerance:      0.1,
			receivedAnswer: "not_a_number",
			expectedValid:  false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := CreateNumericToleranceRule(tt.answer, tt.tolerance)

			isValid, err := validator.ValidateAnswer(rule, tt.receivedAnswer)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && isValid != tt.expectedValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectedValid, isValid)
			}
		})
	}
}

func TestValidator_ValidateRegex(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name           string
		pattern        string
		receivedAnswer string
		expectedValid  bool
		expectError    bool
	}{
		{
			name:           "SimplePattern_Match",
			pattern:        "^[a-z]+$",
			receivedAnswer: "hello",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "SimplePattern_NoMatch",
			pattern:        "^[a-z]+$",
			receivedAnswer: "Hello123",
			expectedValid:  false,
			expectError:    false,
		},
		{
			name:           "EmailPattern_Valid",
			pattern:        `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			receivedAnswer: "test@example.com",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "EmailPattern_Invalid",
			pattern:        `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			receivedAnswer: "invalid-email",
			expectedValid:  false,
			expectError:    false,
		},
		{
			name:           "NumberPattern_Valid",
			pattern:        `^\d+(\.\d+)?$`,
			receivedAnswer: "123.456",
			expectedValid:  true,
			expectError:    false,
		},
		{
			name:           "NumberPattern_Invalid",
			pattern:        `^\d+(\.\d+)?$`,
			receivedAnswer: "123.456.789",
			expectedValid:  false,
			expectError:    false,
		},
		{
			name:           "InvalidPattern",
			pattern:        "[",
			receivedAnswer: "test",
			expectedValid:  false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := CreateRegexRule(tt.pattern)

			isValid, err := validator.ValidateAnswer(rule, tt.receivedAnswer)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && isValid != tt.expectedValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectedValid, isValid)
			}
		})
	}
}

func TestValidator_UnknownValidationType(t *testing.T) {
	validator := NewValidator()

	rule := models.ValidationRule{
		Type:   "UnknownType",
		Answer: "test",
	}

	isValid, err := validator.ValidateAnswer(rule, "test")

	if err == nil {
		t.Error("Expected error for unknown validation type")
	}

	if isValid {
		t.Error("Expected validation to fail for unknown type")
	}

	if err.Error() != "unknown validation rule type: UnknownType" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestValidator_MissingParams(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name string
		rule models.ValidationRule
	}{
		{
			name: "NumericTolerance_MissingParams",
			rule: models.ValidationRule{
				Type:   "NumericTolerance",
				Answer: "42.0",
				Params: nil,
			},
		},
		{
			name: "Regex_MissingParams",
			rule: models.ValidationRule{
				Type:   "Regex",
				Answer: "",
				Params: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid, err := validator.ValidateAnswer(tt.rule, "test")

			if err == nil {
				t.Error("Expected error for missing params")
			}

			if isValid {
				t.Error("Expected validation to fail for missing params")
			}
		})
	}
}

func TestValidator_InvalidParams(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name string
		rule models.ValidationRule
	}{
		{
			name: "NumericTolerance_InvalidParams",
			rule: models.ValidationRule{
				Type:   "NumericTolerance",
				Answer: "42.0",
				Params: json.RawMessage(`invalid_json`),
			},
		},
		{
			name: "Regex_InvalidParams",
			rule: models.ValidationRule{
				Type:   "Regex",
				Answer: "",
				Params: json.RawMessage(`invalid_json`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateAnswer(tt.rule, "test")

			if err == nil {
				t.Error("Expected error for invalid params JSON")
			}
		})
	}
}

func TestCreateExactMatchRule(t *testing.T) {
	tests := []struct {
		name          string
		answer        string
		caseSensitive bool
	}{
		{
			name:          "CaseSensitive",
			answer:        "Test Answer",
			caseSensitive: true,
		},
		{
			name:          "CaseInsensitive",
			answer:        "Test Answer",
			caseSensitive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := CreateExactMatchRule(tt.answer, tt.caseSensitive)

			if rule.Type != "ExactMatch" {
				t.Errorf("Expected type 'ExactMatch', got '%s'", rule.Type)
			}

			if rule.Answer != tt.answer {
				t.Errorf("Expected answer '%s', got '%s'", tt.answer, rule.Answer)
			}

			var params models.ExactMatchParams
			err := json.Unmarshal(rule.Params, &params)
			if err != nil {
				t.Errorf("Failed to unmarshal params: %v", err)
			}

			if params.CaseSensitive != tt.caseSensitive {
				t.Errorf("Expected case sensitive %v, got %v", tt.caseSensitive, params.CaseSensitive)
			}
		})
	}
}

func TestCreateNumericToleranceRule(t *testing.T) {
	answer := "42.5"
	tolerance := 0.1

	rule := CreateNumericToleranceRule(answer, tolerance)

	if rule.Type != "NumericTolerance" {
		t.Errorf("Expected type 'NumericTolerance', got '%s'", rule.Type)
	}

	if rule.Answer != answer {
		t.Errorf("Expected answer '%s', got '%s'", answer, rule.Answer)
	}

	var params models.NumericToleranceParams
	err := json.Unmarshal(rule.Params, &params)
	if err != nil {
		t.Errorf("Failed to unmarshal params: %v", err)
	}

	if params.Tolerance != tolerance {
		t.Errorf("Expected tolerance %v, got %v", tolerance, params.Tolerance)
	}
}

func TestCreateRegexRule(t *testing.T) {
	pattern := "^[a-zA-Z]+$"

	rule := CreateRegexRule(pattern)

	if rule.Type != "Regex" {
		t.Errorf("Expected type 'Regex', got '%s'", rule.Type)
	}

	if rule.Answer != "" {
		t.Errorf("Expected empty answer for regex rule, got '%s'", rule.Answer)
	}

	var params models.RegexParams
	err := json.Unmarshal(rule.Params, &params)
	if err != nil {
		t.Errorf("Failed to unmarshal params: %v", err)
	}

	if params.Pattern != pattern {
		t.Errorf("Expected pattern '%s', got '%s'", pattern, params.Pattern)
	}
}
