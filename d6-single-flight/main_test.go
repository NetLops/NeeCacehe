package main

import (
	"fmt"
	"log"
	"testing"
)

func TestTestIdexSubString(t *testing.T) {
	addr := "httpss://www.netlops.com"
	index := getUrlIndex(addr, -1)
	defer func() {
		if err := recover(); err != nil {
			log.Panicf("The input is incorrect, Origin Error:%v", err)
		}
	}()
	fmt.Println(addr[index:])
}
