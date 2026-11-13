package generic

import (
	"io"
)

var (
	GetBody     = getBody
	GetBodySafe = getBodySafe
)

func getBody(remote string) ([]byte, error) {
	data, err := ReadResource(remote)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	return io.ReadAll(data)
}

// getBodySafe reads an HTTP(S) resource while blocking requests to local and
// private network addresses.
func getBodySafe(remote string) ([]byte, error) {
	data, err := readResourceSafe(remote)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	return io.ReadAll(data)
}
