package router

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// 路由表
type Router struct {
	// 路由表中的最终节点
	To []string `json:"to"`
	// 路由表中的下一跳next节点
	Next []string `json:"next"`
	// 传输到下一节点的花销
	Cost []int `json:"cost"`
	// 路由器节点名称
	Name string `json:"name"`
	// 路由器的通信udp端口
	Port int `json:"port"`
}

func New(name string) (*Router, error) {
	// 初始化路由表To字段
	To := make([]string, routerNumber)
	for i := 0; i < routerNumber; i++ {
		To[i] = routerNames[i]
	}

	// 初始化路由表Next,Cost字段
	Next := make([]string, routerNumber)
	Cost := make([]int, routerNumber)
	for i := 0; i < routerNumber; i++ {
		Cost[i] = DISCONNECT
	}

	Name := name
	Port := -1

	// 在已有配置中查询(数量少直接for遍历)
	for i := 0; i < routerNumber; i++ {
		if strings.Compare(Name, routerNames[i]) == 0 {
			Port = routerPorts[i]
			break
		}
	}

	// 查询不到说明无该节点
	if Port == -1 {
		return nil, ErrInit
	}

	// 自己到自己,next = name, cost = 0
	Next[NodeToIndex[name]] = name
	Cost[NodeToIndex[name]] = 0

	// 根据topology.json的数据初始化路由表数据
	for _, costFromNodeToNode := range costFromNodeToNodes {
		if strings.Compare(Name, costFromNodeToNode.Src) == 0 {
			Next[NodeToIndex[costFromNodeToNode.Dist]] = costFromNodeToNode.Dist
			Cost[NodeToIndex[costFromNodeToNode.Dist]] = costFromNodeToNode.Cost
		}
	}

	return &Router{
		To,
		Next,
		Cost,
		Name,
		Port,
	}, nil
}

// 向其他端口发送消息(udp通信)
func (r *Router) sendMessage(message []byte, port int) error {
	// 本机不发本机
	if r.Port == port {
		return nil
	}

	// 指定目标地址
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", LOCALHOSTIP, port))
	if err != nil {
		return err
	}
	// 指定源地址(为了防止和监听端口起冲突，发送消息的端口使用动态端口(就是指定为0,这样操作系统会自己随机选))
	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:0", LOCALHOSTIP))
	if err != nil {
		return err
	}

	// 建立连接
	conn, err := net.DialUDP("udp", localAddr, serverAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 发送消息
	_, err = conn.Write(message)
	if err != nil {
		return err
	}

	return nil
}

// 发送本机配置消息(节点名+端口号)
func (r *Router) SendConfiguration(port int) error {
	// 本机配置JSON序列化
	message, err := json.Marshal(ConfigurationMessage{
		Name: r.Name,
		Port: r.Port,
	})
	if err != nil {
		return err
	}
	return r.sendMessage(message, port)
}

// 发送本机信息和路由表
func (r *Router) SendRouterTable(port int) error {
	// 路由表JSON序列化
	message, err := json.Marshal(r)
	if err != nil {
		return nil
	}
	return r.sendMessage(message, port)
}

// 监听自己端口传来的消息
func (r *Router) ReceiveMessage(done chan struct{}, wg *sync.WaitGroup, messageQueue []chan Router) {
	// 创建监听端口
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", LOCALHOSTIP, r.Port))
	if err != nil {
		panic(err)
	}

	// 创建UDP端口
	conn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		panic(err)
	}

	wg.Add(1)
	// 使用协程监听处理消息
	// 使用阻塞管道来进行协程的控制
	go func(done chan struct{}, conn *net.UDPConn) {
		// 阻塞主线程让协程结束
		defer wg.Done()
		// 关闭连接
		defer conn.Close()

		// 创建缓冲区
		buffer := make([]byte, BUFFERSIZE)
		for {
			// 使用阻塞线程
			select {
			case <-done: // 主线程通知结束
				logrus.Info(fmt.Sprintf("%s监听完毕", r.Name))
				return
			default:
				// 设置最长响应时间
				// 长时间不读取进行下一轮，防止阻塞
				if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
					logrus.Error("设置阻塞超时时间失败,退出监听")
					return
				}

				n, _, err := conn.ReadFromUDP(buffer)
				if err != nil {
					// 检测是不是超时引起的，是超时引起的跳过本轮即可
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}

					// 不是超时造成的错误
					// 端口无法使用,记录错误
					logrus.Error(err)
					continue
				}
				message := buffer[:n]

				// 解析发过来的报文数据
				// 发送的报文数据不止一种，要分情况进行解析
				// 目前的报文只有两种，一种是配置信息，一种是路由表
				decoder := json.NewDecoder(strings.NewReader(string(message)))
				decoder.DisallowUnknownFields()

				var sendMessage ConfigurationMessage
				err = decoder.Decode(&sendMessage)
				// 解析错误，说明不是配置信息
				if err != nil || len(sendMessage.ExtraFields) != 0 {
					// 按照路由表进行解析
					var router Router
					err = json.Unmarshal(message, &router)

					// 两次解析全部失败,记录日志,直接跳过
					if err != nil {
						logrus.Error(fmt.Sprintf("接收其他路由器消息JSON解析失败,消息%s, err:%v", message, err))
						continue
					}

					// 处理路由表:放入管道
					messageQueue[NodeToIndex[router.Name]] <- router
				} else {
					// 解析正确,说明是配置信息
					// 直接回发本机的路由表即可
					r.SendRouterTable(sendMessage.Port)
				}
			}
			// 按周期监听
			time.Sleep(LISTENCYCLE * time.Second)
		}
	}(done, conn)
}

// 返回路由表的格式化字符串
func (r *Router) RouterTable() string {
	s := fmt.Sprintf("The router table:%s, working on port:%d\n", r.Name, r.Port)
	s += fmt.Sprintf("%-20s%-20s%-20s\n", "To", "Next", "Cost")
	for i := 0; i < routerNumber; i++ {
		s += fmt.Sprintf("%-20s%-20s%-20d\n", r.To[i], r.Next[i], r.Cost[i])
	}
	return s
}

// 处理路由表
func (r *Router) distanceVectorRoutingAlgorithmOnRouter(otherNouter Router) {
	// 如果之前断连，先更新A->B
	if r.Cost[NodeToIndex[otherNouter.Name]] == DISCONNECT {
		for _, costFromNodeToNode := range costFromNodeToNodes {
			if strings.Compare(r.Name, costFromNodeToNode.Src) == 0 && strings.Compare(otherNouter.Name, costFromNodeToNode.Dist) == 0 {
				r.Cost[NodeToIndex[otherNouter.Name]] = costFromNodeToNode.Cost
				break
			}
		}
	}
	// 处理路由表
	for i := 0; i < routerNumber; i++ {
		// 自己到自己不用处理
		if strings.Compare(r.Name, routerNames[i]) == 0 {
			continue
		}

		// A->dist, B->dist
		myToDist, otherToDist := r.Cost[i], otherNouter.Cost[i]

		// 全断连不处理
		if myToDist == DISCONNECT && otherToDist == DISCONNECT {
			continue
		} else if myToDist != DISCONNECT && otherToDist == DISCONNECT {
			// A->dist连通,B->dist断连
			if strings.Compare(r.Next[i], otherNouter.Name) == 0 {
				// 但是A的下一跳就是B,更新数据(也是断连)
				r.Cost[i] = DISCONNECT
			}
		} else if myToDist == DISCONNECT && otherToDist != DISCONNECT {
			// A->dist断,B->dist通
			// A.Next = B
			r.Next[i] = otherNouter.Name
			// A->dist = A->B + B->dist
			r.Cost[i] = r.Cost[NodeToIndex[otherNouter.Name]] + otherToDist
		} else {
			// A->dist通.B->dist通
			// A->B + B->dist
			myToOtherToDist := r.Cost[NodeToIndex[otherNouter.Name]] + otherToDist
			// 取最小
			if myToOtherToDist < myToDist {
				r.Next[i] = otherNouter.Name
				r.Cost[i] = myToOtherToDist
			}
		}
	}
}

// 处理断连节点
func (r *Router) HandleDisconnectNode(otherNode string) {
	for i := 0; i < routerNumber; i++ {
		if strings.Compare(r.Next[i], otherNode) == 0 {
			r.Cost[i] = DISCONNECT
		}
	}
}
