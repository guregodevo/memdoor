package gateway

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// outputSchemaRetries is how many times a reply whose JSON does not match a
// task's output_schema is sent back before the task fails.
const outputSchemaRetries = 2

var jsonFence = regexp.MustCompile("(?s)```json\\s*(.*?)```")

// matchOutputSchema finds the reply's JSON (the last ```json block, else the
// last top-level object) and checks it against the schema. It returns the
// JSON, compacted, as the task's output.
func matchOutputSchema(schema, reply string) (string, error) {
	doc := ""
	if m := jsonFence.FindAllStringSubmatch(reply, -1); len(m) > 0 {
		doc = strings.TrimSpace(m[len(m)-1][1])
	} else if i, j := strings.Index(reply, "{"), strings.LastIndex(reply, "}"); i >= 0 && j > i {
		doc = reply[i : j+1]
	}
	if doc == "" {
		return "", fmt.Errorf("no JSON object in the reply")
	}
	var v any
	if err := json.Unmarshal([]byte(doc), &v); err != nil {
		return "", fmt.Errorf("not valid JSON: %v", err)
	}
	res, err := gojsonschema.Validate(gojsonschema.NewStringLoader(schema), gojsonschema.NewGoLoader(v))
	if err != nil {
		return "", fmt.Errorf("the schema itself is invalid: %v", err)
	}
	if !res.Valid() {
		var why []string
		for _, e := range res.Errors() {
			why = append(why, e.String())
		}
		return "", fmt.Errorf("%s", strings.Join(why, "; "))
	}
	out, _ := json.Marshal(v)
	return string(out), nil
}
