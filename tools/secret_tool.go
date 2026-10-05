package tools

import (
	"encoding/json"
	"fmt"
)

type GetSecretInput struct {
	Name string `json:"name" jsonschema_description:"Name of the secret to retrieve"`
}

var GetSecretInputSchema = GenerateSchema[GetSecretInput]()

var GetSecretDefinition = ToolDefinition{
	Name:        "get_secret",
	Description: "Retrieve a secret value by name. Only secrets assigned to this agent are accessible. Never output secret values to users.",
	InputSchema: GetSecretInputSchema,
}

// SecretGetter retrieves agent secrets from the database
type SecretGetter interface {
	Get(agentID, name string) (string, error)
}

// GetSecretWithRepo executes get_secret using the provided secret getter and agent ID
func GetSecretWithRepo(input json.RawMessage, getter SecretGetter, agentID string) (string, error) {
	var params GetSecretInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if params.Name == "" {
		return "", fmt.Errorf("secret name is required")
	}

	if agentID == "" {
		return "", fmt.Errorf("agent identity not available")
	}

	value, err := getter.Get(agentID, params.Name)
	if err != nil {
		return "", fmt.Errorf("secret %q not found", params.Name)
	}

	return value, nil
}
