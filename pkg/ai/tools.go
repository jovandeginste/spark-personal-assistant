package ai

import (
	"fmt"
	"strings"
)

const maxToolArgLengthLog = 40

// FormatToolArgs formats tool arguments for display in notices and logs.
func FormatToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}

	argList := make([]string, 0, len(args))
	for k, v := range args {
		value := fmt.Sprintf("%v", v)
		if len(value) > maxToolArgLengthLog {
			value = value[:maxToolArgLengthLog] + "..."
		}

		argList = append(argList, fmt.Sprintf("%s: %q", k, value))
	}

	return "(" + strings.Join(argList, ", ") + ")"
}
