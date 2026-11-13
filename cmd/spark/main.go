package main

import (
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
)

func main() {
	a := app.NewApp()

	cmd := NewCLI(a)

	err := cmd.rootCmd.Execute()

	a.Close()

	if err != nil {
		os.Exit(1)
	}
}
