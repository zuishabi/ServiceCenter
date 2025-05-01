package ZSC

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zuishabi/ServiceCenter/src/msg"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"sync"
	"time"
)

type Client struct {
	IP                          string
	Port                        int
	Name                        string
	InterestingService          []string
	ServiceCenterAddr           string
	OnInterestingServiceOnline  func(info *ServiceStatus)
	OnInterestingServiceOffline func(string)
	OnCenterDisconnected        func()
	TimeoutTime                 int //向服务器查询的超时时间
	conn                        *net.TCPConn
	getServiceStatusBufferLock  sync.Mutex
	getServiceStatusBuffer      map[string]chan ServiceStatus
	connLock                    sync.Mutex
}

type ServiceStatus struct {
	Name   string
	IP     string
	Port   int
	Status uint32
}

func NewClient(ip string, port int, name string, interestingService []string, serviceCenterAddr string) *Client {
	return &Client{
		IP:                     ip,
		Port:                   port,
		Name:                   name,
		InterestingService:     interestingService,
		ServiceCenterAddr:      serviceCenterAddr,
		getServiceStatusBuffer: make(map[string]chan ServiceStatus),
		TimeoutTime:            1000,
	}
}

// Start 开始与服务注册中心的连接
func (c *Client) Start() error {
	addr, err := net.ResolveTCPAddr("tcp", c.ServiceCenterAddr)
	if err != nil {
		return err
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}
	c.conn = conn
	d := json.NewEncoder(conn)
	serviceInfo := ServiceInfo{
		IP:                 c.IP,
		Port:               c.Port,
		Name:               c.Name,
		InterestingService: c.InterestingService,
	}
	if err := d.Encode(&serviceInfo); err != nil {
		return err
	}
	go c.startHeartBeat()
	go c.startRead()
	return nil
}

// RegisterOnlineFunc 注册当感兴趣的服务上线时触发的回调函数
func (c *Client) RegisterOnlineFunc(f func(info *ServiceStatus)) {
	c.OnInterestingServiceOnline = f
}

// RegisterOfflineFunc 注册当感兴趣的服务下线时所触发的回调函数
func (c *Client) RegisterOfflineFunc(f func(string)) {
	c.OnInterestingServiceOffline = f
}

// RegisterCenterDisconnectedFunc 注册当与注册中心断开连接后触发的回调函数
func (c *Client) RegisterCenterDisconnectedFunc(f func()) {
	c.OnCenterDisconnected = f
}

// SetTimeoutTime 设置客户端向服务器交互的超时时间，默认为1000mil
func (c *Client) SetTimeoutTime(time int) {
	c.TimeoutTime = time
}

// StopService 关闭当前服务
func (c *Client) StopService() {
	_ = c.conn.Close()
}

// GetServiceStatus 向注册中心获得一个服务的当前状态信息
func (c *Client) GetServiceStatus(name string) (ServiceStatus, error) {
	m := msg.ServiceStatusRequest{Name: name}
	data, _ := proto.Marshal(&m)
	if err := c.sendMsg(3, data); err != nil {
		return ServiceStatus{}, err
	}
	wait := make(chan ServiceStatus)
	c.registerGetServiceStatus(name, wait)
	select {
	case res := <-wait:
		return res, nil
	case <-time.After(time.Duration(c.TimeoutTime) * time.Millisecond):
		c.deleteGetServiceStatus(name)
		return ServiceStatus{}, errors.New("timeout")
	}
}

// 开始传输心跳包
func (c *Client) startHeartBeat() {
	h := msg.HeartBeat{Name: c.Name}
	data, _ := proto.Marshal(&h)
	for {
		select {
		case <-time.After(time.Second * 2):
			_ = c.sendMsg(1, data)
		}
	}
}

func (c *Client) sendMsg(id uint32, msg []byte) error {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	msgID := make([]byte, 4)
	binary.BigEndian.PutUint32(msgID, id)
	if _, err := c.conn.Write(msgID); err != nil {
		return err
	}
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msg)))
	if _, err := c.conn.Write(msgLen); err != nil {
		return err
	}
	if _, err := c.conn.Write(msg); err != nil {
		return err
	}
	return nil
}

// 处理所接收到的信息
func (c *Client) processMsg(id uint32, m []byte) {
	switch id {
	case 2:
		info := msg.ServiceInfo{}
		err := proto.Unmarshal(m, &info)
		if err != nil {
			fmt.Println(err)
		}
		if info.Status == 1 && c.OnInterestingServiceOnline != nil {
			c.OnInterestingServiceOnline(&ServiceStatus{
				Name:   info.Name,
				IP:     info.Ip,
				Port:   int(info.Port),
				Status: info.Status,
			})
		} else if info.Status == 2 && c.OnInterestingServiceOffline != nil {
			c.OnInterestingServiceOffline(info.Name)
		}
	case 3:
		status := msg.ServiceInfo{}
		err := proto.Unmarshal(m, &status)
		if err != nil {
			fmt.Println(err)
		}
		c.getServiceStatusBufferLock.Lock()
		defer c.getServiceStatusBufferLock.Unlock()
		fmt.Println(2)
		if ch := c.getServiceStatusBuffer[status.Name]; ch != nil {
			fmt.Println(3)
			ch <- ServiceStatus{
				IP:     status.Ip,
				Port:   int(status.Port),
				Name:   status.Name,
				Status: status.Status,
			}
			fmt.Println(4)
		}
	default:
		fmt.Println("unknown msg id = ", id)
	}
}

func (c *Client) startRead() {
	for {
		msgID := make([]byte, 4)
		if _, err := io.ReadFull(c.conn, msgID); err != nil {
			fmt.Println("get msg id error,err = ", err)
			break
		}
		id := binary.BigEndian.Uint32(msgID)
		msgLen := make([]byte, 4)
		if _, err := io.ReadFull(c.conn, msgLen); err != nil {
			fmt.Println("get msg len error,err = ", err)
			break
		}
		length := binary.BigEndian.Uint32(msgLen)
		m := make([]byte, length)
		if _, err := io.ReadFull(c.conn, m); err != nil {
			fmt.Println("get msg error,err = ", err)
			break
		}
		c.processMsg(id, m)
	}
}

func (c *Client) registerGetServiceStatus(name string, ch chan ServiceStatus) {
	c.getServiceStatusBufferLock.Lock()
	defer c.getServiceStatusBufferLock.Unlock()
	c.getServiceStatusBuffer[name] = ch
	fmt.Println(1)
}

func (c *Client) deleteGetServiceStatus(name string) {
	c.getServiceStatusBufferLock.Lock()
	defer c.getServiceStatusBufferLock.Unlock()
	close(c.getServiceStatusBuffer[name])
	delete(c.getServiceStatusBuffer, name)
}
