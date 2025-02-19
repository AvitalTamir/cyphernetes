package core

import (
	"fmt"
	"strings"
	"time"
)

// TemporalHandler handles temporal operations and comparisons
type TemporalHandler struct{}

// NewTemporalHandler creates a new TemporalHandler
func NewTemporalHandler() *TemporalHandler {
	return &TemporalHandler{}
}

// EvaluateTemporalExpression evaluates a temporal expression and returns a time.Time
func (h *TemporalHandler) EvaluateTemporalExpression(expr *TemporalExpression) (time.Time, error) {
	fmt.Printf("Evaluating temporal expression: Function=%s, Operation=%s\n", expr.Function, expr.Operation)

	switch expr.Function {
	case "datetime":
		// Get current time once and use it consistently
		now := time.Now().UTC().Truncate(time.Second)
		if expr.Operation != "" {
			if expr.RightExpr == nil {
				return time.Time{}, fmt.Errorf("right expression required for datetime operation")
			}
			if expr.RightExpr.Function != "duration" {
				return time.Time{}, fmt.Errorf("datetime operations only supported with duration")
			}

			// For datetime() - duration(), we need to evaluate the duration first
			duration, err := h.ParseISO8601Duration(expr.RightExpr.Argument)
			if err != nil {
				return time.Time{}, err
			}

			var result time.Time
			switch expr.Operation {
			case "+":
				result = now.Add(duration)
			case "-":
				result = now.Add(-duration)
			default:
				return time.Time{}, fmt.Errorf("unsupported temporal operation: %s", expr.Operation)
			}
			fmt.Printf("Datetime operation result: %v (now=%v, duration=%v)\n", result, now, duration)
			return result, nil
		}
		return now, nil
	case "duration":
		// For duration function, we need to parse the ISO 8601 duration and apply it
		duration, err := h.ParseISO8601Duration(expr.Argument)
		if err != nil {
			return time.Time{}, err
		}
		fmt.Printf("Parsed duration: %v\n", duration)

		// If there's an operation, apply it to the right expression
		if expr.Operation != "" {
			rightTime, err := h.EvaluateTemporalExpression(expr.RightExpr)
			if err != nil {
				return time.Time{}, err
			}

			var result time.Time
			switch expr.Operation {
			case "+":
				result = rightTime.Add(duration)
			case "-":
				result = rightTime.Add(-duration)
			default:
				return time.Time{}, fmt.Errorf("unsupported temporal operation: %s", expr.Operation)
			}
			fmt.Printf("Duration operation result: %v\n", result)
			return result, nil
		}

		// If no operation, just return current time minus duration
		now := time.Now().UTC().Truncate(time.Second)
		result := now.Add(-duration)
		fmt.Printf("Simple duration result: %v\n", result)
		return result, nil
	default:
		return time.Time{}, fmt.Errorf("unsupported temporal function: %s", expr.Function)
	}
}

// ParseISO8601Duration parses an ISO 8601 duration string and returns a time.Duration
func (h *TemporalHandler) ParseISO8601Duration(durationStr string) (time.Duration, error) {
	if !strings.HasPrefix(durationStr, "P") {
		return 0, fmt.Errorf("invalid ISO 8601 duration format: must start with P")
	}

	var duration time.Duration
	var number string
	var inTimeSection bool

	for i := 1; i < len(durationStr); i++ {
		char := durationStr[i]

		if char == 'T' {
			inTimeSection = true
			continue
		}

		if char >= '0' && char <= '9' {
			number += string(char)
			continue
		}

		if number == "" {
			return 0, fmt.Errorf("invalid ISO 8601 duration format: no number before designator")
		}

		value := 0
		if _, err := fmt.Sscanf(number, "%d", &value); err != nil {
			return 0, fmt.Errorf("invalid number in duration: %s", number)
		}

		switch char {
		case 'Y':
			if inTimeSection {
				return 0, fmt.Errorf("invalid ISO 8601 duration format: Y not allowed in time section")
			}
			duration += time.Duration(value) * 24 * 365 * time.Hour
		case 'M':
			if inTimeSection {
				duration += time.Duration(value) * time.Minute
			} else {
				duration += time.Duration(value) * 24 * 30 * time.Hour
			}
		case 'D':
			if inTimeSection {
				return 0, fmt.Errorf("invalid ISO 8601 duration format: D not allowed in time section")
			}
			duration += time.Duration(value) * 24 * time.Hour
		case 'H':
			if !inTimeSection {
				return 0, fmt.Errorf("invalid ISO 8601 duration format: H only allowed in time section")
			}
			duration += time.Duration(value) * time.Hour
		case 'S':
			if !inTimeSection {
				return 0, fmt.Errorf("invalid ISO 8601 duration format: S only allowed in time section")
			}
			duration += time.Duration(value) * time.Second
		default:
			return 0, fmt.Errorf("invalid ISO 8601 duration format: unknown designator %c", char)
		}

		number = ""
	}

	if number != "" {
		return 0, fmt.Errorf("invalid ISO 8601 duration format: number without designator")
	}

	return duration, nil
}

// CompareTemporalValues compares a resource value with a temporal expression
func (h *TemporalHandler) CompareTemporalValues(resourceTime time.Time, expr *TemporalExpression, operator string) (bool, error) {
	compareTime, err := h.EvaluateTemporalExpression(expr)
	if err != nil {
		return false, err
	}

	// Truncate both times to second precision for consistent comparison
	resourceTime = resourceTime.Truncate(time.Second)
	compareTime = compareTime.Truncate(time.Second)

	fmt.Printf("Temporal comparison: Resource time: %v, Compare time: %v, Operator: %s\n", resourceTime, compareTime, operator)

	// Note: The comparison is resourceTime <operator> compareTime
	// For example: creationTimestamp < datetime() - duration("PT1H")
	// means: if the resource was created before the comparison time
	switch operator {
	case "EQUALS":
		return resourceTime.Equal(compareTime), nil
	case "NOT_EQUALS":
		return !resourceTime.Equal(compareTime), nil
	case "GREATER_THAN":
		return resourceTime.After(compareTime), nil
	case "LESS_THAN":
		// For "less than", we want to check if the resource time is before the compare time
		// e.g., if pod was created before (datetime() - 1h)
		return resourceTime.Before(compareTime), nil
	case "GREATER_THAN_EQUALS":
		return resourceTime.After(compareTime) || resourceTime.Equal(compareTime), nil
	case "LESS_THAN_EQUALS":
		return resourceTime.Before(compareTime) || resourceTime.Equal(compareTime), nil
	default:
		return false, fmt.Errorf("unsupported temporal operator: %s", operator)
	}
}
