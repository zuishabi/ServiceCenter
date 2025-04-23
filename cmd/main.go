package main

import "ServiceCenter/src"

func main() {
	s := src.NewServiceCenter("127.0.0.1", 9999, 5)
	s.Start()
}
