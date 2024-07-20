package main

import (
	"log"

	"github.com/vkupriya/go-gophermart/internal/gophermart"
)

func main() {
	if err := gophermart.Start(); err != nil {
		log.Fatal(err)
	}
	log.Println("gophermart server stopped.")
}
