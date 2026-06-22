package main

import (
	"fmt"
	"os"

	"github.com/wau/profile/service"
)

func main() {
	if err := service.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "wau-profile-service failed: %v\n", err)
		os.Exit(1)
	}
}
