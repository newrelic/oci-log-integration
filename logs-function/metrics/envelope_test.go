package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExtractEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		record   map[string]interface{}
		expected Envelope
	}{
		{
			name: "prefers oracle.ingestedtime over source time",
			record: map[string]interface{}{
				"time": "2023-01-01T12:00:00Z",
				"oracle": map[string]interface{}{
					"ingestedtime": "2023-01-01T12:00:05Z",
				},
			},
			expected: Envelope{
				LagTime:    time.Date(2023, 1, 1, 12, 0, 5, 0, time.UTC),
				HasLagTime: true,
			},
		},
		{
			name: "falls back to source time when ingestedtime absent",
			record: map[string]interface{}{
				"time": "2023-01-01T12:00:00Z",
			},
			expected: Envelope{
				LagTime:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				HasLagTime: true,
			},
		},
		{
			name: "falls back to source time when ingestedtime malformed",
			record: map[string]interface{}{
				"time": "2023-01-01T12:00:00Z",
				"oracle": map[string]interface{}{
					"ingestedtime": "not-a-time",
				},
			},
			expected: Envelope{
				LagTime:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				HasLagTime: true,
			},
		},
		{
			name:     "nil record",
			record:   nil,
			expected: Envelope{},
		},
		{
			name:     "empty record",
			record:   map[string]interface{}{},
			expected: Envelope{},
		},
		{
			name: "malformed time is ignored",
			record: map[string]interface{}{
				"time": "not-a-time",
			},
			expected: Envelope{},
		},
		{
			name: "non-string time is ignored",
			record: map[string]interface{}{
				"time": 12345,
			},
			expected: Envelope{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ExtractEnvelope(tt.record))
		})
	}
}
