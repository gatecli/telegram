package main

import (
	"fmt"
	"os"

	"github.com/gatecli/gatecli"
)

func main() {
	service := NewTelegramService()
	app, err := gatecli.Create(service)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
