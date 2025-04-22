package main

import (
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
)

func main() {
	a := app.NewApp()

	cmd := NewCLI(a)
	if err := cmd.rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
