package main

import (
	"io"
	"net/http"
)

func getBody(remote string) ([]byte, error) {
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
