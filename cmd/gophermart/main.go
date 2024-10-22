package main

import (
	"github.com/nglmq/gofermart-loyalty-programm/internal/config"
	"github.com/nglmq/gofermart-loyalty-programm/internal/http-server/server"
	"log"
	"net/http"
)

func main() {
	r, err := server.Start()
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServe(config.RunAddr, r))
}
