package main

import ZSC "github.com/zuishabi/ServiceCenter/src"

func main() {
	s := ZSC.NewServiceCenter("127.0.0.1", 9999, 5)
	s.Start()
	select {}
}
