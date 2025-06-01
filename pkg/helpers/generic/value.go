package generic

import (
	"strconv"
	"strings"
)

func CleanValue(value any) any {
	v, ok := value.(string)
	if !ok {
		return value
	}

	if uv, err := strconv.Unquote("\"" + v + "\""); err == nil {
		return strings.TrimSpace(uv)
	}

	return strings.TrimSpace(v)
}
