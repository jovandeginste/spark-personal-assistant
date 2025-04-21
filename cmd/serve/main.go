package main

import "github.com/jovandeginste/spark-personal-assistant/pkg/app"

func main() {
	a := app.NewApp()
	if err := a.Initialize(); err != nil {
		panic(err)
	}

	es, err := a.CurrentEntries()
	if err != nil {
		panic(err)
	}

	if err := a.GeneratePrompt(es); err != nil {
		panic(err)
	}
}
