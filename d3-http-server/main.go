package main

import (
	"fmt"
	"log"
	"neecache"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func main() {
	neecache.NewGroup("scores", 2<<10, neecache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
	addr := ":9999"
	peers := neecache.NewHTTPPool(addr)
	log.Println("neecache is running at", addr)
	log.Fatal(http.ListenAndServe(addr, peers))
}
