package main

import (
	"context"
	"log"
)

func main() {
	config, err := configFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	engine, cleanup, err := newDemoApp(context.Background(), config)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = cleanup(context.Background())
	}()

	log.Printf("claw-sdk-gomvc demo listening on %s", config.HTTPAddr)
	if err := engine.Run(config.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
