package ZSC

import (
	"fmt"
	"testing"
)

func TestClient(t *testing.T) {
	s := NewServiceCenter("127.0.0.1", 9999, 5)
	s.Start()
	c1 := NewClient("127.0.0.1", 8999, "MainServer", []string{"CloudStore"}, "127.0.0.1:9999")
	if err := c1.Start(); err != nil {
		fmt.Println(err)
	}
	if a, err := c1.GetServiceStatus("CloudStore"); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(a)
	}
	if a, err := c1.GetServiceStatus("MainServer"); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(a)
	}
	c2 := NewClient("127.0.0.1", 8998, "CloudStore", nil, "127.0.0.1:9999")
	if err := c2.Start(); err != nil {
		fmt.Println(err)
	}
	if a, err := c1.GetServiceStatus("CloudStore"); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(a)
	}
	c2.StopService()
	if a, err := c1.GetServiceStatus("CloudStore"); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(a)
	}
}
