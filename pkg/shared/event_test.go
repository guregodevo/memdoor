package shared

import (
	"testing"
)

// These tests verify Event functionality (unit tests)
// For cross-context integration tests, see integration_test.go

func TestEventType_GetCategory(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      EventCategory
	}{
		{EventExecutionStarted, EventCategoryLifecycle},
		{EventExecutionCompleted, EventCategoryLifecycle},
		{EventThinkingStarted, EventCategoryThinking},
		{EventThinkingCompleted, EventCategoryThinking},
		{EventToolCallStarted, EventCategoryTool},
		{EventToolCallCompleted, EventCategoryTool},
		{EventResponseGenerated, EventCategoryResponse},
		{EventErrorOccurred, EventCategoryError},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			got := tt.eventType.GetCategory()
			if got != tt.want {
				t.Errorf("GetCategory() = %v, want %v", got, tt.want)
			}
		})
	}
}
