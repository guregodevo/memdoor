package consumer

import (
	"encoding/json"
	"fmt"

	"memdoor/gateway/infra"
)

// Discriminator parses raw event data based on stream type
// Pattern: Discriminated unions (TypeScript) → Go type switches
type Discriminator struct {
	verbose bool
}

// NewDiscriminator creates a new event data discriminator
func NewDiscriminator(verbose bool) *Discriminator {
	return &Discriminator{
		verbose: verbose,
	}
}

// ParseEventData parses event.Data into typed structures based on event.Stream
// Returns ParsedEventData interface that can be type-asserted to specific types
func (d *Discriminator) ParseEventData(event infra.AgentEvent) (ParsedEventData, error) {
	stream := ConvertInfraStream(event.Stream)

	switch stream {
	case StreamLifecycle:
		return d.parseLifecycleData(event.Data)
	case StreamTool:
		return d.parseToolData(event.Data)
	case StreamAssistant:
		return d.parseAssistantData(event.Data)
	case StreamError:
		return d.parseErrorData(event.Data)
	case StreamContext:
		return d.parseContextData(event.Data)
	default:
		return nil, fmt.Errorf("unknown stream type: %s", stream)
	}
}

// parseLifecycleData parses lifecycle event data
func (d *Discriminator) parseLifecycleData(data map[string]interface{}) (LifecycleEventData, error) {
	// Marshal to JSON and unmarshal to struct (handles type conversions)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return LifecycleEventData{}, fmt.Errorf("failed to marshal lifecycle data: %w", err)
	}

	var lifecycleData LifecycleEventData
	if err := json.Unmarshal(jsonData, &lifecycleData); err != nil {
		return LifecycleEventData{}, fmt.Errorf("failed to unmarshal lifecycle data: %w", err)
	}

	return lifecycleData, nil
}

// parseToolData parses tool event data
func (d *Discriminator) parseToolData(data map[string]interface{}) (ToolEventData, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ToolEventData{}, fmt.Errorf("failed to marshal tool data: %w", err)
	}

	var toolData ToolEventData
	if err := json.Unmarshal(jsonData, &toolData); err != nil {
		return ToolEventData{}, fmt.Errorf("failed to unmarshal tool data: %w", err)
	}

	return toolData, nil
}

// parseAssistantData parses assistant event data
func (d *Discriminator) parseAssistantData(data map[string]interface{}) (AssistantEventData, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return AssistantEventData{}, fmt.Errorf("failed to marshal assistant data: %w", err)
	}

	var assistantData AssistantEventData
	if err := json.Unmarshal(jsonData, &assistantData); err != nil {
		return AssistantEventData{}, fmt.Errorf("failed to unmarshal assistant data: %w", err)
	}

	return assistantData, nil
}

// parseErrorData parses error event data
func (d *Discriminator) parseErrorData(data map[string]interface{}) (ErrorEventData, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ErrorEventData{}, fmt.Errorf("failed to marshal error data: %w", err)
	}

	var errorData ErrorEventData
	if err := json.Unmarshal(jsonData, &errorData); err != nil {
		return ErrorEventData{}, fmt.Errorf("failed to unmarshal error data: %w", err)
	}

	return errorData, nil
}

// parseContextData parses context event data
func (d *Discriminator) parseContextData(data map[string]interface{}) (ContextEventData, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ContextEventData{}, fmt.Errorf("failed to marshal context data: %w", err)
	}

	var contextData ContextEventData
	if err := json.Unmarshal(jsonData, &contextData); err != nil {
		return ContextEventData{}, fmt.Errorf("failed to unmarshal context data: %w", err)
	}

	return contextData, nil
}

// Helper functions for extracting common fields

// GetEventType extracts the "event" field from parsed data
func GetEventType(data ParsedEventData) string {
	switch d := data.(type) {
	case LifecycleEventData:
		return d.Event
	case ToolEventData:
		return d.Event
	case AssistantEventData:
		return d.Event
	case ErrorEventData:
		return d.Event
	case ContextEventData:
		return d.Event
	default:
		return ""
	}
}
