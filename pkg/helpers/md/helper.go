package md

import (
	"io"

	"github.com/jovandeginste/spark-personal-assistant/pkg/markdown"
	stripmd "github.com/writeas/go-strip-markdown"
)

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

func MDToText(file io.Reader) (string, error) {
	md, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return stripmd.Strip(string(md)), nil
}
