package generic

import (
	"io"
	"net/http"
)

func GetBody(remote string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, remote, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Spark")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	return io.ReadAll(res.Body)
}
