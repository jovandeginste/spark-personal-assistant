package main

import (
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
)

// Create cobra configuration to create an entry

func main() {
	a := app.NewApp()
	if err := a.Initialize(); err != nil {
		panic(err)
	}

	cmd := NewCLI(a)
	if err := cmd.rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
