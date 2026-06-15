package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ing-bank/golibs/pkg/graceful"
)

func app(ctx context.Context) error {
	for range 10 {
		select {
		case <-ctx.Done():
			return nil
		default:
			log.Println("press ctrl-c to exit gracefully")
			time.Sleep(1 * time.Second)
		}
	}
	return fmt.Errorf("no response from user")
}

func main() {
	if err := graceful.Run(context.Background(), app); err != nil {
		log.Fatal("application did not exit gracefully:", err)
	}
}
