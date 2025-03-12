package core

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func convertToComparableTypes(result, filterValue interface{}) (interface{}, interface{}, error) {
	// Handle null value comparisons
	if filterValue == nil {
		return result, nil, nil
	}

	// If both are already the same type, return them as is
	if reflect.TypeOf(result) == reflect.TypeOf(filterValue) {
		return result, filterValue, nil
	}

	// Try to convert both to float64 for numeric comparisons
	resultFloat, resultErr := toFloat64(result)
	filterFloat, filterErr := toFloat64(filterValue)

	if resultErr == nil && filterErr == nil {
		return resultFloat, filterFloat, nil
	}

	// If conversion to float64 failed, convert both to strings
	return fmt.Sprintf("%v", result), fmt.Sprintf("%v", filterValue), nil
}

func toFloat64(v interface{}) (float64, error) {
	switch v := v.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %v to float64", v)
	}
}

// convertToMilliCPU converts a CPU value string to milliCPU (integer format).
// It handles both standard CPU values (e.g., "1", "0.5") and milliCPU values (e.g., "500m").
func convertToMilliCPU(cpu string) (int, error) {
	// Check if the value is in milliCPU format
	if strings.HasSuffix(cpu, "m") {
		// Trim the "m" suffix and convert the remaining string to an integer
		milliCPU, err := strconv.Atoi(strings.TrimSuffix(cpu, "m"))
		if err != nil {
			return 0, fmt.Errorf("invalid milliCPU value: %s", cpu)
		}
		return milliCPU, nil
	}

	// Convert to base unit (milliCPU) if no "m" suffix
	standardCPU, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid standard CPU value: %s", cpu)
	}

	return int(standardCPU * 1000), nil
}

// convertMilliCPUToStandard converts a CPU value in milliCPU to the standard notation (integer or float)
// if it's more readable. If it's less than 1000m, it returns the value in milliCPU format.
func convertMilliCPUToStandard(milliCPU int) string {
	// If the value is 1000m or greater, convert to standard CPU notation
	if milliCPU >= 1000 {
		// Convert to standard CPU by dividing by 1000 and check for decimal points
		standardCPU := float64(milliCPU) / 1000.0

		// If the value is a whole number (e.g., 2000m becomes 2), format as an integer
		if standardCPU == float64(int(standardCPU)) {
			return strconv.Itoa(int(standardCPU))
		}

		// Otherwise, format as a float, then drop the unnecessary trailing 0's
		standardCPU_str := strings.TrimRight(fmt.Sprintf("%.3f", standardCPU), "0")

		if strings.HasSuffix(standardCPU_str, ".") {
			standardCPU_str = strings.TrimRight(standardCPU_str, ".")
		}

		return standardCPU_str
	}

	// If less than 1000m, return the value in milliCPU format with the "m" suffix
	return strconv.Itoa(milliCPU) + "m"
}

// convertMemoryToBytes takes a memory string like "500M" or "2Gi"
// and returns the corresponding value in bytes.
func convertMemoryToBytes(mem string) (int64, error) {
	// Suffixes for power-of-10 (decimal) memory units in Kubernetes
	suffixesDecimal := map[string]int64{
		"E": 1e18, // Exabyte
		"P": 1e15, // Petabyte
		"T": 1e12, // Terabyte
		"G": 1e9,  // Gigabyte
		"M": 1e6,  // Megabyte
		"k": 1e3,  // Kilobyte (lowercase for kilobytes in decimal)
	}

	// Suffixes for power-of-2 (binary) memory units
	suffixesBinary := map[string]int64{
		"Ei": 1 << 60, // Exbibyte (2^60)
		"Pi": 1 << 50, // Pebibyte (2^50)
		"Ti": 1 << 40, // Tebibyte (2^40)
		"Gi": 1 << 30, // Gibibyte (2^30)
		"Mi": 1 << 20, // Mebibyte (2^20)
		"Ki": 1 << 10, // Kibibyte (2^10)
	}

	// Check for power-of-2 suffixes first (Ei, Pi, Ti, etc.)
	for suffix, multiplier := range suffixesBinary {
		if strings.HasSuffix(mem, suffix) {
			numberStr := strings.TrimSuffix(mem, suffix)
			number, err := strconv.ParseFloat(numberStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number format: %v", err)
			}
			return int64(number * float64(multiplier)), nil
		}
	}

	// Check for power-of-10 suffixes next (E, P, T, etc.)
	for suffix, multiplier := range suffixesDecimal {
		if strings.HasSuffix(mem, suffix) {
			numberStr := strings.TrimSuffix(mem, suffix)
			number, err := strconv.ParseFloat(numberStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number format: %v", err)
			}
			return int64(number * float64(multiplier)), nil
		}
	}

	// If no suffix is found, assume it's in bytes
	number, err := strconv.ParseFloat(mem, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory format: %v", err)
	}
	return int64(number), nil
}

// convertBytesToMemory converts a value in bytes to the closest readable unit,
// supporting both decimal (e.g., kB, MB) and binary (e.g., KiB, MiB) units.
func convertBytesToMemory(bytes int64) string {
	// Binary units (power-of-2)
	binaryUnits := []struct {
		suffix     string
		multiplier int64
	}{
		{"Ei", 1 << 60}, // Exbibyte
		{"Pi", 1 << 50}, // Pebibyte
		{"Ti", 1 << 40}, // Tebibyte
		{"Gi", 1 << 30}, // Gibibyte
		{"Mi", 1 << 20}, // Mebibyte
		{"Ki", 1 << 10}, // Kibibyte
	}

	// Decimal units (power-of-10)
	decimalUnits := []struct {
		suffix     string
		multiplier int64
	}{
		{"E", 1e18}, // Exabyte
		{"P", 1e15}, // Petabyte
		{"T", 1e12}, // Terabyte
		{"G", 1e9},  // Gigabyte
		{"M", 1e6},  // Megabyte
		{"k", 1e3},  // Kilobyte
	}

	// First check for decimal units (power-of-10) exactly
	for _, unit := range decimalUnits {
		if bytes == unit.multiplier {
			return fmt.Sprintf("1%s", unit.suffix)
		}
	}

	// Then check for binary units (power-of-two)
	for _, unit := range binaryUnits {
		if bytes >= unit.multiplier {
			value := float64(bytes) / float64(unit.multiplier)
			formatted := strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
			return fmt.Sprintf("%s%s", formatted, unit.suffix)
		}
	}

	// If no binary unit applies, check for decimal units (power-of-ten)
	for _, unit := range decimalUnits {
		if bytes >= unit.multiplier {
			value := float64(bytes) / float64(unit.multiplier)
			formatted := strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
			return fmt.Sprintf("%s%s", formatted, unit.suffix)
		}
	}

	// If no unit applies, return the value in bytes
	return fmt.Sprintf("%d", bytes)
}

func convertToStringSlice(v1 reflect.Value) ([]string, error) {
	if v1.Kind() != reflect.Slice {
		return nil, fmt.Errorf("input is not a slice")
	}

	length := v1.Len()
	result := make([]string, length)

	for i := 0; i < length; i++ {
		elem := v1.Index(i)

		// If the element is directly a string
		if elem.Kind() == reflect.String {
			result[i] = elem.String()
		} else {
			// Try to convert the element to a string
			if elem.CanInterface() {
				switch v := elem.Interface().(type) {
				case string:
					result[i] = v
				case fmt.Stringer:
					result[i] = v.String()
				default:
					// As a last resort, use fmt.Sprint
					result[i] = fmt.Sprint(v)
				}
			} else {
				return nil, fmt.Errorf("cannot convert element at index %d to string", i)
			}
		}
	}

	return result, nil
}

func sumMilliCPU(cpuStrs []string) (int, error) {
	cpuSum := 0
	for _, cpuStr := range cpuStrs {
		cpuVal, err := convertToMilliCPU(cpuStr)
		if err != nil {
			return 0, fmt.Errorf("error processing CPU value: %v", err)
		}

		cpuSum += cpuVal
	}

	return cpuSum, nil
}

func sumMemoryBytes(memStrs []string) (int64, error) {
	memSum := int64(0)
	for _, memStr := range memStrs {
		memVal, err := convertMemoryToBytes(memStr)
		if err != nil {
			return 0, fmt.Errorf("error processing Memory value: %v", err)
		}

		memSum += memVal
	}

	return memSum, nil
}
