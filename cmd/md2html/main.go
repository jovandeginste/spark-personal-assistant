package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/markdown"
)

func main() {
	file := os.Stdin
	if len(os.Args) >= 2 {
		var err error

		file, err = os.Open(os.Args[1])
		if err != nil {
			panic(err)
		}
	}

	if err := mdToHTML(file); err != nil {
		panic(err)
	}
}

func mdToHTML(file io.Reader) error {
	md, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	html, err := markdown.GenerateHTML(md)
	if err != nil {
		return err
	}

	fmt.Println(string(html))

	return nil
}
