package ZSC

import (
	"fmt"
	"github.com/zuishabi/ServiceCenter/src/msg"
	"google.golang.org/protobuf/proto"
	"net"
	"sync"
	"time"
)

type ServiceCenter struct {
	IP                  string
	Port                int
	heartbeatTime       int //当服务超过多久没有进行心跳检测后将此服务标记为关闭
	statusLock          sync.Mutex
	serviceStatus       map[string]*Service //存放所有服务的状态
	interestingLock     sync.Mutex
	interestingServices map[string]map[string]*Service //存放服务所对应的感兴趣的服务
}

func NewServiceCenter(ip string, port int, heartbeatTime int) *ServiceCenter {
	return &ServiceCenter{
		IP:                  ip,
		Port:                port,
		heartbeatTime:       heartbeatTime,
		serviceStatus:       make(map[string]*Service),
		interestingServices: make(map[string]map[string]*Service),
	}
}

// Start 启动服务注册中心
func (s *ServiceCenter) Start() {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", s.IP, s.Port))
	if err != nil {
		fmt.Println("resolve tcp addr error,err = ", err)
		return
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		fmt.Println("listen tcp error,err = ", err)
		return
	}
	go func() {
		for {
			conn, err := l.AcceptTCP()
			if err != nil {
				fmt.Println("accept tcp conn error,err = ", err)
				continue
			}
			go newService(conn, s).start()
		}
	}()
}

// UpdateHeartBeat 当收到来自服务的心跳包后，更新心跳状态
func (s *ServiceCenter) updateHeartBeat(name string) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.serviceStatus[name].heartbeatTime = time.Now()
}

func (s *ServiceCenter) checkServiceStatus() {
	for {
		select {
		case <-time.After(time.Second * 5):
			s.statusLock.Lock()
			for name, service := range s.serviceStatus {
				if time.Since(service.heartbeatTime) > time.Second*5 {
					fmt.Printf("Service %s missed heartbeat", name)
					s.interestingLock.Lock()
					//先将其所有感兴趣的服务从感兴趣列表中删除
					for _, v := range s.serviceStatus[name].InterestingService {
						delete(s.interestingServices[v], name)
					}
					//关闭连接
					_ = s.serviceStatus[name].conn.Close()
					//接着将服务从服务状态列表中删除
					delete(s.serviceStatus, name)
					//接着在感兴趣列表中寻找此服务并通知所有当前在线的对此服务感兴趣的所有服务
					status := msg.ServiceInfo{
						Name:   name,
						Status: 2,
					}
					m, _ := proto.Marshal(&status)
					for _, v := range s.interestingServices[name] {
						if v == nil {
							continue
						}
						if err := v.sendMsg(2, m); err != nil {
							fmt.Println("send offline msg error,err = ", err)
						}
					}
					s.interestingLock.Unlock()
				}
			}
			s.statusLock.Unlock()
		}
	}
}

// 当一个服务下线时调用
func (s *ServiceCenter) offlineService(name string) {
	fmt.Println("service offline : ", name)
	s.statusLock.Lock()
	s.interestingLock.Lock()
	defer s.statusLock.Unlock()
	defer s.interestingLock.Unlock()
	//先将其所有感兴趣的服务从感兴趣列表中删除
	for _, v := range s.serviceStatus[name].InterestingService {
		delete(s.interestingServices[v], name)
	}
	//关闭连接
	_ = s.serviceStatus[name].conn.Close()
	//接着将服务从服务状态列表中删除
	delete(s.serviceStatus, name)
	//接着在感兴趣列表中寻找此服务并通知所有当前在线的对此服务感兴趣的所有服务
	status := msg.ServiceInfo{
		Name:   name,
		Status: 2,
	}
	m, _ := proto.Marshal(&status)
	//如果当前服务没有服务对其感兴趣，直接返回
	if s.interestingServices[name] == nil {
		return
	}
	for _, v := range s.interestingServices[name] {
		if v == nil {
			continue
		}
		if err := v.sendMsg(2, m); err != nil {
			fmt.Println("send offline msg error,err = ", err)
		}
	}
}

// 注册一个服务
func (s *ServiceCenter) registerService(service *Service) {
	s.statusLock.Lock()
	s.interestingLock.Lock()
	defer s.statusLock.Unlock()
	defer s.interestingLock.Unlock()
	s.serviceStatus[service.Name] = service
	for _, v := range service.InterestingService {
		if s.interestingServices[v] == nil {
			s.interestingServices[v] = make(map[string]*Service)
		}
		s.interestingServices[v][service.Name] = service
	}
}

// 查询一个服务是否在线
func (s *ServiceCenter) getServiceStatus(name string) uint32 {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	if s.serviceStatus[name] == nil {
		return 2
	}
	return 1
}
