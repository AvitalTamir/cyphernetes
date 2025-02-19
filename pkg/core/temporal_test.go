package core

import (
	"testing"
	"time"
)

func TestTemporalHandler(t *testing.T) {
	handler := NewTemporalHandler()

	t.Run("EvaluateTemporalExpression", func(t *testing.T) {
		tests := []struct {
			name    string
			expr    *TemporalExpression
			want    time.Time
			wantErr bool
		}{
			{
				name: "current time with datetime()",
				expr: &TemporalExpression{
					Function: "datetime",
				},
				wantErr: false,
			},
			{
				name: "specific time with datetime()",
				expr: &TemporalExpression{
					Function: "datetime",
					Argument: "2024-02-19T10:00:00Z",
				},
				want:    time.Date(2024, 2, 19, 10, 0, 0, 0, time.UTC),
				wantErr: false,
			},
			{
				name: "invalid datetime format",
				expr: &TemporalExpression{
					Function: "datetime",
					Argument: "2024-02-19", // Invalid format
				},
				wantErr: true,
			},
			{
				name: "datetime() - duration(PT1H)",
				expr: &TemporalExpression{
					Function:  "datetime",
					Operation: "-",
					RightExpr: &TemporalExpression{
						Function: "duration",
						Argument: "PT1H",
					},
				},
				wantErr: false,
			},
			{
				name: "datetime() + duration(P1D)",
				expr: &TemporalExpression{
					Function:  "datetime",
					Operation: "+",
					RightExpr: &TemporalExpression{
						Function: "duration",
						Argument: "P1D",
					},
				},
				wantErr: false,
			},
			{
				name: "specific datetime - duration",
				expr: &TemporalExpression{
					Function:  "datetime",
					Argument:  "2024-02-19T10:00:00Z",
					Operation: "-",
					RightExpr: &TemporalExpression{
						Function: "duration",
						Argument: "PT2H",
					},
				},
				want:    time.Date(2024, 2, 19, 8, 0, 0, 0, time.UTC),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := handler.EvaluateTemporalExpression(tt.expr)
				if (err != nil) != tt.wantErr {
					t.Errorf("EvaluateTemporalExpression() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr {
					if tt.want.IsZero() {
						// For current time tests or relative time operations,
						// verify the result is within a reasonable range
						now := time.Now().UTC()

						if tt.expr.Operation != "" && tt.expr.RightExpr != nil && tt.expr.RightExpr.Function == "duration" {
							// Parse the expected duration
							expectedDuration, err := handler.ParseISO8601Duration(tt.expr.RightExpr.Argument)
							if err != nil {
								t.Errorf("Failed to parse duration: %v", err)
								return
							}

							var expectedTime time.Time
							if tt.expr.Operation == "-" {
								expectedTime = now.Add(-expectedDuration)
							} else { // "+"
								expectedTime = now.Add(expectedDuration)
							}

							// Allow for a small timing difference (5 seconds)
							diff := got.Sub(expectedTime)
							if diff < -5*time.Second || diff > 5*time.Second {
								t.Errorf("EvaluateTemporalExpression() result = %v, expected around %v (diff: %v)", got, expectedTime, diff)
							}
						} else {
							// For current time, verify it's within 5 seconds
							diff := now.Sub(got)
							if diff < -5*time.Second || diff > 5*time.Second {
								t.Errorf("EvaluateTemporalExpression() time difference too large: %v", diff)
							}
						}
					} else if !got.Equal(tt.want) {
						t.Errorf("EvaluateTemporalExpression() = %v, want %v", got, tt.want)
					}
				}
			})
		}
	})

	t.Run("ParseISO8601Duration", func(t *testing.T) {
		tests := []struct {
			name     string
			duration string
			want     time.Duration
			wantErr  bool
		}{
			{
				name:     "one hour",
				duration: "PT1H",
				want:     time.Hour,
				wantErr:  false,
			},
			{
				name:     "one day",
				duration: "P1D",
				want:     24 * time.Hour,
				wantErr:  false,
			},
			{
				name:     "complex duration",
				duration: "P1DT2H30M",
				want:     26*time.Hour + 30*time.Minute,
				wantErr:  false,
			},
			{
				name:     "invalid format - missing P",
				duration: "1D",
				wantErr:  true,
			},
			{
				name:     "invalid format - no number",
				duration: "PT",
				wantErr:  true,
			},
			{
				name:     "invalid format - wrong order",
				duration: "P1H2D",
				wantErr:  true,
			},
			{
				name:     "invalid format - empty string",
				duration: "",
				wantErr:  true,
			},
			{
				name:     "invalid format - just P",
				duration: "P",
				wantErr:  true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := handler.ParseISO8601Duration(tt.duration)
				if (err != nil) != tt.wantErr {
					t.Errorf("ParseISO8601Duration() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("ParseISO8601Duration() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("CompareTemporalValues", func(t *testing.T) {
		now := time.Now().UTC()
		oneHourAgo := now.Add(-time.Hour)
		twoHoursAgo := now.Add(-2 * time.Hour)

		tests := []struct {
			name         string
			resourceTime time.Time
			expr         *TemporalExpression
			operator     string
			want         bool
			wantErr      bool
		}{
			{
				name:         "less than one hour ago",
				resourceTime: oneHourAgo,
				expr: &TemporalExpression{
					Function:  "datetime",
					Operation: "-",
					RightExpr: &TemporalExpression{
						Function: "duration",
						Argument: "PT1H",
					},
				},
				operator: "LESS_THAN",
				want:     false,
				wantErr:  false,
			},
			{
				name:         "greater than one hour ago",
				resourceTime: twoHoursAgo,
				expr: &TemporalExpression{
					Function:  "datetime",
					Operation: "-",
					RightExpr: &TemporalExpression{
						Function: "duration",
						Argument: "PT1H",
					},
				},
				operator: "LESS_THAN",
				want:     true,
				wantErr:  false,
			},
			{
				name:         "equals specific time",
				resourceTime: time.Date(2024, 2, 19, 10, 0, 0, 0, time.UTC),
				expr: &TemporalExpression{
					Function: "datetime",
					Argument: "2024-02-19T10:00:00Z",
				},
				operator: "EQUALS",
				want:     true,
				wantErr:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := handler.CompareTemporalValues(tt.resourceTime, tt.expr, tt.operator)
				if (err != nil) != tt.wantErr {
					t.Errorf("CompareTemporalValues() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("CompareTemporalValues() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}
