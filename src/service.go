package ZSC

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/zuishabi/ServiceCenter/src/msg"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"sync"
	"time"
)

// Service 定义具体的服务，保存在服务注册中心中
type Service struct {
	connLock      sync.Mutex
	conn          *net.TCPConn
	center        *ServiceCenter
	heartbeatTime time.Time
	ServiceInfo
}

type ServiceInfo struct {
	IP                 string
	Port               int
	Name               string
	InterestingService []string
}

func newService(conn *net.TCPConn, center *ServiceCenter) *Service {
	return &Service{
		conn:          conn,
		center:        center,
		ServiceInfo:   ServiceInfo{},
		heartbeatTime: time.Now(),
	}
}

// 启动服务的监听
func (s *Service) start() {
	//先解析出服务所传来的信息
	e := json.NewDecoder(s.conn)
	err := e.Decode(&s.ServiceInfo)
	if err != nil {
		fmt.Println("get service info error,err = ", err)
		return
	}
	s.center.registerService(s)
	for {
		msgID := make([]byte, 4)
		if _, err := io.ReadFull(s.conn, msgID); err != nil {
			fmt.Println("get msg id error,err = ", err)
			break
		}
		id := binary.BigEndian.Uint32(msgID)
		msgLen := make([]byte, 4)
		if _, err := io.ReadFull(s.conn, msgLen); err != nil {
			fmt.Println("get msg len error,err = ", err)
			break
		}
		length := binary.BigEndian.Uint32(msgLen)
		m := make([]byte, length)
		if _, err := io.ReadFull(s.conn, m); err != nil {
			fmt.Println("get msg error,err = ", err)
			break
		}
		s.processMsg(id, m)
	}
	s.center.offlineService(s.Name)
}

// 向服务写入消息
func (s *Service) sendMsg(id uint32, msg []byte) error {
	s.connLock.Lock()
	defer s.connLock.Unlock()
	msgID := make([]byte, 4)
	binary.BigEndian.PutUint32(msgID, id)
	if _, err := s.conn.Write(msgID); err != nil {
		return err
	}
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msg)))
	if _, err := s.conn.Write(msgLen); err != nil {
		return err
	}
	if _, err := s.conn.Write(msg); err != nil {
		return err
	}
	return nil
}

// 处理收到的信息
func (s *Service) processMsg(id uint32, m []byte) {
	switch id {
	case 1:
		heartbeat := msg.HeartBeat{}
		_ = proto.Unmarshal(m, &heartbeat)
		s.center.updateHeartBeat(s.Name)
	case 3:
		fmt.Println("a")
		inquiry := msg.ServiceStatusRequest{}
		_ = proto.Unmarshal(m, &inquiry)
		res := &msg.ServiceInfo{}
		res = s.center.getServiceStatus(inquiry.Name)
		data, _ := proto.Marshal(res)
		if err := s.sendMsg(3, data); err != nil {
			fmt.Println("send status info error,err = ", err)
			return
		}
	}
}
