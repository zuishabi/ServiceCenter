package ZSC

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/zuishabi/ServiceCenter/src/msg"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"time"
)

type Client struct {
	IP                 string
	port               int
	Name               string
	InterestingService []string
	ServiceCenterAddr  string
	*net.TCPConn
	OnInterestingServiceOnline  func(info *msg.ServiceInfo)
	OnInterestingServiceOffline func(string)
}

func NewClient(ip string, port int, name string, interestingService []string, serviceCenterAddr string) *Client {
	return &Client{
		IP:                 ip,
		port:               port,
		Name:               name,
		InterestingService: interestingService,
		ServiceCenterAddr:  serviceCenterAddr,
	}
}

// RegisterOnlineFunc 注册当感兴趣的服务上线时触发的回调函数
func (c *Client) RegisterOnlineFunc(f func(info *msg.ServiceInfo)) {
	c.OnInterestingServiceOnline = f
}

// RegisterOfflineFunc 注册当感兴趣的服务下线时所触发的回调函数
func (c *Client) RegisterOfflineFunc(f func(string)) {
	c.OnInterestingServiceOffline = f
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
	c.TCPConn = conn
	d := json.NewEncoder(conn)
	serviceInfo := ServiceInfo{
		IP:                 c.IP,
		Port:               c.port,
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

func (c *Client) startRead() {
	for {
		msgID := make([]byte, 4)
		if _, err := io.ReadFull(c, msgID); err != nil {
			fmt.Println("get msg id error,err = ", err)
			break
		}
		id := binary.BigEndian.Uint32(msgID)
		msgLen := make([]byte, 4)
		if _, err := io.ReadFull(c, msgLen); err != nil {
			fmt.Println("get msg len error,err = ", err)
			break
		}
		length := binary.BigEndian.Uint32(msgLen)
		m := make([]byte, length)
		if _, err := io.ReadFull(c, m); err != nil {
			fmt.Println("get msg error,err = ", err)
			break
		}
		c.processMsg(id, m)
	}
}

// 处理所接收到的信息
func (c *Client) processMsg(id uint32, m []byte) {
	switch id {
	case 2:
		info := msg.ServiceInfo{}
		_ = proto.Unmarshal(m, &info)
		if info.Status == 1 {
			c.OnInterestingServiceOnline(&info)
		} else if info.Status == 2 {
			c.OnInterestingServiceOffline(info.Name)
		}
	default:
		fmt.Println("unknown msg id = ", id)
	}
}

func (c *Client) SendMsg(id uint32, msg []byte) error {
	msgID := make([]byte, 4)
	binary.BigEndian.PutUint32(msgID, id)
	if _, err := c.TCPConn.Write(msgID); err != nil {
		return err
	}
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msg)))
	if _, err := c.TCPConn.Write(msgLen); err != nil {
		return err
	}
	if _, err := c.TCPConn.Write(msg); err != nil {
		return err
	}
	return nil
}

// StopService 关闭当前服务
func (c *Client) StopService() {
	_ = c.Close()
}

// 开始传输心跳包
func (c *Client) startHeartBeat() {
	h := msg.HeartBeat{Name: c.Name}
	data, _ := proto.Marshal(&h)
	for {
		select {
		case <-time.After(time.Second * 2):
			_ = c.SendMsg(1, data)
		}
	}
}
