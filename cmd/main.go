package main

import (
	ZSC "github.com/zuishabi/ServiceCenter/src"
)

func main() {
	s := ZSC.NewServiceCenter("0.0.0.0", 9999, 5)
	s.Start()
	select {}
}
