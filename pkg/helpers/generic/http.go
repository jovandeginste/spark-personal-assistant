package generic

import (
	"io"
)

var GetBody = getBody

func getBody(remote string) ([]byte, error) {
	data, err := ReadResource(remote)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(data)
}
