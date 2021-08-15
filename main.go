package main

import (
	"fmt"
	"log"
	"net"

	"canhui.wang/factory/boot"
	"canhui.wang/factory/inotify"
)

func main1() {
	boot.PowerOn()
}

func main() {
	w, _ := inotify.NewNetlinkWatcher()

	go func() {
		for msg := range w.Wait() {
			if msg.IsNewAddr() {
				log.Printf("%+v\n", msg)
			}
		}
	}()

	ifces, _ := net.Interfaces()
	for _, ifce := range ifces {
		log.Printf("%+v\n", ifce)
		addrs, _ := ifce.Addrs()
		for _, addr := range addrs {
			log.Printf("%+v\n", addr)
		}
	}

	ifce, _ := net.InterfaceByName("veth31e6ec2")
	ifce.Addrs()

	var a int
	fmt.Scan(&a)
}
