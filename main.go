package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"time"
)

/*
	注册中心v0.2
	使用tcp连接
	服务向服务中心注册时会传递自身的名称，ip，以及所感兴趣的服务
	同时服务注册中心会将感兴趣的服务注册到表中，当对应的服务状态更新时及时通知所感兴趣的服务
	使用protobuf作为通讯协议。
*/

type Service struct {
	Addr     string
	Port     uint32
	Name     string
	beatTime time.Time
}

type ServiceManager struct {
	ServiceList map[string]*Service
}

var ServiceMgr *ServiceManager

func (s *ServiceManager) RegisterService(service Service, reply *int) error {
	log.Println("Service Register success," + service.Name)
	service.beatTime = time.Now()
	s.ServiceList[service.Name] = &service
	return nil
}

func (s *ServiceManager) GetService(name string, reply *Service) error {
	if service, ok := s.ServiceList[name]; ok {
		fmt.Println(service)
		*reply = *service
		return nil
	}
	return errors.New("not find service")
}

func (s *ServiceManager) CheckHeartBeat(service Service, reply *int) error {
	if s, ok := s.ServiceList[service.Name]; ok {
		s.beatTime = time.Now()
		log.Println("get heartbeat success", service.Name)
		return nil
	} else {
		return errors.New("not found service")
	}
}

func startHeartBeat() {
	for {
		select {
		case <-time.After(time.Second * 5):
			for name, s := range ServiceMgr.ServiceList {
				if time.Since(s.beatTime) > time.Second*5 {
					log.Printf("Service %s missed heartbeat", name)
					delete(ServiceMgr.ServiceList, name)
				}
			}
		}
	}
}

func main() {
	ServiceMgr = &ServiceManager{ServiceList: make(map[string]*Service)}
	err := rpc.Register(ServiceMgr)
	if err != nil {
		log.Println("Server Register error = ", err)
		return
	}
	go startHeartBeat()
	l, err := net.Listen("tcp", "service-center:9999")
	if err != nil {
		panic(err)
	}
	rpc.Accept(l)
}
