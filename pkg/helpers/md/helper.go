package md

import (
	"io"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/markdown"
)

func MDFileToHTML(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}

	return MDToHTML(file)
}

func MDToHTML(file io.Reader) (string, error) {
	md, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	html, err := markdown.GenerateHTML(md)
	if err != nil {
		return "", err
	}

	return string(html), nil
}
