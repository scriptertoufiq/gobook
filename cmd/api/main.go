// Command api is the HTTP server entrypoint.
package main

import (
	"log"

	"github.com/scriptertoufiq/gobook/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("boot failed: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("runtime error: %v", err)
	}
}
