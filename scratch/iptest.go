package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	ief, err := net.InterfaceByName("wlan0")
	if err != nil {
		log.Fatalf("error getting interface by name: %s", err)
	}

	addrs, err := ief.Addrs()
	if err != nil {
		log.Fatalf("error getting addresses for interface: %s", err)
	}
	for _, addr := range addrs {
		fmt.Printf("addr: %s:%s\n", addr.Network(), addr.String())
		// addr: ip+net:192.168.86.253/24
		// addr: ip+net:fd64:9f44:fc30:0:b951:8b16:2812:a227/64
		// addr: ip+net:fe80::2cc9:801b:3551:9a43/64
	}
}
