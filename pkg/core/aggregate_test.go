package core

import (
	"reflect"
	"testing"
)

func TestConvertToMilliCPU(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{"Valid milliCPU", "100m", 100, false},
		{"Valid standard CPU", "1", 1000, false},
		{"Valid standard CPU", "2", 2000, false},
		{"Valid standard CPU", "1.5", 1500, false},
		{"Valid standard CPU", "3.7", 3700, false},
		{"Valid standard CPU", "0.1", 100, false},
		{"Valid standard CPU", "1.234", 1234, false},
		{"Valid standard CPU", "0.001", 1, false},
		{"Invalid milliCPU", "100x", 0, true},
		{"invalid format", "abc", 0, true},
		{"Zero milliCPU", "0m", 0, false},
		{"Zero standard CPU", "0", 0, false},
		{"Empty string", "", 0, true},
		{"milliCPU format", "500m", 500, false},
		{"milliCPU format", "1000m", 1000, false},
		{"large number in standard CPU", "100", 100000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToMilliCPU(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToMilliCPU() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("convertToMilliCPU() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConvertMilliCPUToStandard(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"Less than 1000m", 500, "500m"},
		{"Exactly 1000m", 1000, "1"},
		{"More than 1000m, whole number", 2000, "2"},
		{"More than 1000m, decimal", 1500, "1.5"},
		{"Large number", 5678, "5.678"},
		{"Large number", 12340, "12.34"},
		{"Zero", 0, "0m"},
		{"Very small number", 1, "1m"},
		{"Very large number", 10000, "10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMilliCPUToStandard(tt.input)
			if result != tt.expected {
				t.Errorf("convertMilliCPUToStandard(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertMemoryToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{"Exabyte", "1E", 1e18, false},
		{"Petabyte", "1P", 1e15, false},
		{"Terabyte", "1T", 1e12, false},
		{"Gigabyte", "1G", 1e9, false},
		{"Megabyte", "1M", 1e6, false},
		{"Kilobyte", "1k", 1e3, false},
		{"Exbibyte", "1Ei", 1 << 60, false},
		{"Pebibyte", "1Pi", 1 << 50, false},
		{"Tebibyte", "1Ti", 1 << 40, false},
		{"Gibibyte", "1Gi", 1 << 30, false},
		{"Mebibyte", "1Mi", 1 << 20, false},
		{"Kibibyte", "1Ki", 1 << 10, false},
		{"Bytes", "1000", 1000, false},
		{"Invalid", "1X", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertMemoryToBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertMemoryToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("convertMemoryToBytes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConvertBytesToMemory(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		// Binary (power-of-two) units
		{"Exbibyte", 1 << 60, "1Ei"},
		{"Pebibyte", 1 << 50, "1Pi"},
		{"Tebibyte", 1 << 40, "1Ti"},
		{"Gibibyte", 1 << 30, "1Gi"},
		{"Mebibyte", 1 << 20, "1Mi"},
		{"Kibibyte", 1 << 10, "1Ki"},
		{"Exabyte", 1e18, "1E"},
		{"Petabyte", 1e15, "1P"},
		{"Terabyte", 1e12, "1T"},
		{"Gigabyte", 1e9, "1G"},
		{"Megabyte", 1e6, "1M"},
		{"Kilobyte", 1e3, "1k"},
		{"Zero bytes", 0, "0"},
		{"One byte", 1, "1"},
		{"999 bytes", 999, "999"},
		{"1.5 Gibibytes", 1610612736, "1.5Gi"},
		{"1.5 Mebibytes", 1572864, "1.5Mi"},
		{"1.5 Kibibytes", 1536, "1.5Ki"},
		{"Large binary", 1125899906842624, "1Pi"},
		{"Large decimal", 1000000000000000, "1P"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertBytesToMemory(tt.input)
			if result != tt.expected {
				t.Errorf("convertBytesToMemory(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    reflect.Value
		expected []string
		wantErr  bool
	}{
		{
			name:     "String slice",
			input:    reflect.ValueOf([]string{"a", "b", "c"}),
			expected: []string{"a", "b", "c"},
			wantErr:  false,
		},
		{
			name:     "Int slice",
			input:    reflect.ValueOf([]int{1, 2, 3}),
			expected: []string{"1", "2", "3"},
			wantErr:  false,
		},
		{
			name:     "Not a slice",
			input:    reflect.ValueOf("not a slice"),
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToStringSlice(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToStringSlice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("convertToStringSlice() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSumMilliCPU(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected int
		wantErr  bool
	}{
		{"Valid milliCPU", []string{"100m", "200m", "300m"}, 600, false},
		{"Valid standard CPU", []string{"1", "2", "3"}, 6000, false},
		{"Mixed formats", []string{"1", "2000m", "3"}, 6000, false},
		{"Invalid input", []string{"1", "2", "invalid"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sumMilliCPU(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sumMilliCPU() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("sumMilliCPU() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSumMemoryBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected int64
		wantErr  bool
	}{
		{"Valid memory units", []string{"1Gi", "2Mi", "3Ki"}, 1075842048, false},
		{"Mixed units 1", []string{"1G", "2M", "3k"}, 1002003000, false},
		{"Mixed units 2", []string{"1Ei", "2Ti", "3Mi", "1P", "2G", "3k"}, 1153923705633251256, false},
		{"Mixed units 3", []string{"1Pi", "2Gi", "3Ki", "1E", "2T", "3M"}, 1001127902057329344, false},
		{"Bytes", []string{"1000", "2000", "3000"}, 6000, false},
		{"Invalid input", []string{"1Gi", "2Mi", "invalid", "5K"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sumMemoryBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sumMemoryBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("sumMemoryBytes() = %v, want %v", got, tt.expected)
			}
		})
	}
}
