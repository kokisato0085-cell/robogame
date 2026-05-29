// Command server は RoboGame の REST API サーバーを起動する。
package main

import (
	"log"
	"net/http"

	"robogame/server/api"
	"robogame/server/store"
)

func main() {
	s := store.New()
	handler := api.NewServer(s)

	const addr = ":8080"
	log.Printf("RoboGame API listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
