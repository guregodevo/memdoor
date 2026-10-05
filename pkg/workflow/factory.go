package workflow

import (
	"context"
	"fmt"

	"github.com/guregodevo/mario/tasks"
	"github.com/guregodevo/mario/templates"
)

// TaskRun is what the Runner is handed for one agent task: the rendered
// prompt, the agent, and where to work.
type TaskRun struct {
	Workflow  string
	Partition string
	Task      Task
	Prompt    string
	Workdir   string
}

// Runner does one agent task: one bounded turn of the named agent. The
// answer is kept as the task's proof when the task names no target of its
// own. Declared here so the gateway satisfies it without this package
// knowing it.
type Runner interface {
	Run(ctx context.Context, t TaskRun) (answer string, err error)
}

// TypeAgent is the task type Memdoor brings to mario: a turn of an agent.
// The other types a DAG may mix in — command, llm — are mario's own.
const TypeAgent = "agent"

const defaultAgent = "coder"

// Agent is mario's task type for an agent turn, built on tasks.Base: the
// YAML says `type: agent`, mario builds the task, and the one thing Memdoor
// adds is the turn itself through the Runner.
func Agent(runner Runner) tasks.Base {
	return tasks.Base{Type: TypeAgent, SchemaDoc: []byte(agentSchema), Run: func(ctx context.Context, b *tasks.Base, d *templates.YamlTaskDefinition, name string) (string, error) {
		if runner == nil {
			return "", fmt.Errorf("task %s: no agent runs on this host", name)
		}
		prompt, err := b.Prompt(d)
		if err != nil {
			return "", fmt.Errorf("task %s: prompt template: %w", name, err)
		}
		t := taskOf(name, d)
		t.Name = lastSegment(name)
		return runner.Run(ctx, TaskRun{Workflow: segment(name, 0), Partition: b.Partition, Task: t, Prompt: prompt, Workdir: b.Dir})
	}}
}

const agentSchema = `{
  "description": "One turn of a Memdoor agent, done when its target exists",
  "type": "object",
  "properties": {
    "type": {"enum": ["agent"]},
    "agent": {"type": "string", "description": "Who runs it: coder, planner, runner, verifier … (default coder)."},
    "model": {"type": "string", "description": "Which model answers this task's turn, as /model names them (groq:qwen/qwen3.8-27b, claude-haiku-4-5-20251001); default: the agent's ladder."},
    "prompt": {"type": "string", "description": "The task, as a message to the agent. A Go template over args and partition."},
    "budget": {"type": "integer", "description": "Tokens this task may spend running on alone while its target is missing (default: the workspace's turn_token_budget)."},
    "output_schema": {"type": "object", "description": "A JSON Schema the task's answer must match; the answer is kept as that JSON for the next steps."},
    "tools": {"type": "array", "items": {"type": "string"}, "description": "Reserved: tools the turn may use."},
    "table_description": {"type": "string"},
    "dataset_description": {"type": "string"},
    "field_descriptions": {"type": "object"},
    "args": {"type": "object", "description": "Values the prompt template can use."},
    "requires": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "table_pattern": {"type": "string", "pattern": "^[a-zA-Z0-9_-]+$", "description": "The task required, by name."},
          "project_id": {"type": "string", "description": "Another workflow's name (default: this one)."},
          "dataset_id": {"type": "string", "description": "Another group (default: this task's)."},
          "external": {"type": "boolean", "description": "Made outside this workflow — a person's approval, another system's file. Never run, only checked."}
        },
        "required": ["table_pattern"],
        "additionalProperties": false
      }
    },
    "target": {
      "type": "object",
      "description": "What proves the task done. Without one, the agent's answer is written as the proof.",
      "properties": {
        "file": {"type": "string", "description": "A file that must exist under the project."},
        "command": {"type": "string", "description": "A command that must exit 0 in the project."}
      },
      "additionalProperties": false
    },
    "timeout": {"type": "string", "description": "A clock on the turn, e.g. 20m."},
    "max_retries": {"type": "integer", "minimum": 0},
    "start_date": {"type": "string"},
    "stop_date": {"type": "string"}
  },
  "required": ["type", "prompt"],
  "additionalProperties": false
}`
