package main

import (
	"flag"
	"fmt"
	"log"
	"neecache"
	"net/http"
	"strings"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *neecache.Group {
	return neecache.NewGroup("sources", 2<<10, neecache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		},
	))
}

func getUrlIndex(addr string, index int) int {
	if index = strings.Index(addr, "http://"); index != -1 {
		index = index + 7
	} else if index = strings.Index(addr, "https://"); index != -1 {
		index += 8
	}
	return index
}

// startCacheServer 用来启动缓存服务器：创建HTTPPOOL， 添加节点信息，注册到nee中
// 启动HTTP服务（共3个端口，8001/8002/8003）， 用户不感知
func startCacheServer(addr string, addrs []string, nee *neecache.Group) {
	peers := neecache.NewHTTPPool(addr)
	peers.Set(addrs...)
	nee.RegisterPeers(peers)
	log.Println("neecache is running at", addr)
	index := getUrlIndex(addr, -1)
	defer func() {
		if err := recover(); err != nil {
			log.Panicf("The input is incorrect, Origin Error:%v", err)
		}
	}()
	log.Fatal(http.ListenAndServe(addr[index:], peers))
}

// startAPIServer 用来启动一个API服务（端口9999），与用户进行交互，用户感知
func startAPIServer(apiAddr string, nee *neecache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := nee.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, err = w.Write(view.ByteSlice())
			if err != nil {
				return
			}
		}))
	log.Println("fontend server is running at", apiAddr)
	index := getUrlIndex(apiAddr, -1)
	defer func() {
		if err := recover(); err != nil {
			log.Panicf("The input is incorrect, Origin Error:%v", err)
		}
	}()
	log.Fatal(http.ListenAndServe(apiAddr[index:], nil))
}
func main() {
	var (
		port int
		api  bool
	)
	flag.IntVar(&port, "port", 8001, "Neecache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	apiAddr := "http://localhost:9999"
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}
	nee := createGroup()
	if api {
		go startAPIServer(apiAddr, nee)
	}
	startCacheServer(addrMap[port], []string(addrs), nee)

}
