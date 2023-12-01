package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// 路由节点初始化错误
	ErrInit error = errors.New("路由节点初始化错误")
)

const (
	// 路由和端口的配置文件
	NodeAddrConfigFilePath string = "config/nodeaddr.json"

	// 路由节点之间传输成本cost的配置文件
	NodeCostConfigFilePath string = "config/topology.json"

	// 两个节点断开连接
	DISCONNECT int = -1

	// 本机ip
	LOCALHOSTIP string = "127.0.0.1"

	// 缓冲区大小
	BUFFERSIZE int = 1024

	// 监听周期(s)
	LISTENCYCLE time.Duration = time.Duration(1)

	// 处理周期(s)
	HANDLECYCLE time.Duration = time.Duration(3)

	// 发送后的最长等待时间(s)
	WAITTIME time.Duration = time.Duration(10)

	// 消息队列最大长度
	MESSAGEQUEUEMAXLENGTH int = 10
)

// 路由节点之间的通信代价结构体
type CostFromNodeToNode struct {
	Src  string `json:"src"`
	Dist string `json:"dist"`
	Cost int    `json:"cost"`
}

// 发送本机配置消息的格式(节点名+端口号)
type ConfigurationMessage struct {
	Name        string          `json:"name"`
	Port        int             `json:"port"`
	ExtraFields json.RawMessage `json:"-"`
}

var (
	// 节点数目
	routerNumber int

	// 节点名称
	routerNames []string = make([]string, 0)

	// 节点端口
	routerPorts []int = make([]int, 0)

	// 节点到index的映射
	NodeToIndex map[string]int = make(map[string]int)

	// 端口到index的映射
	PortToIndex map[int]int = make(map[int]int)

	// 路由节点之间的通信开销
	costFromNodeToNodes []CostFromNodeToNode

	// 发送，接收消息的消息队列
	sendAndSaveMessageQueue []chan Router
)

// 读取配置文件并且解析
func InitConfig() {
	nodeAddrData, err := os.ReadFile(NodeAddrConfigFilePath)
	if err != nil {
		logrus.Error("读取配置文件失败!")
		os.Exit(-1)
	}

	// 节点:端口
	type NodeAddr struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}

	var nodeAddrs []NodeAddr
	err = json.Unmarshal(nodeAddrData, &nodeAddrs)
	if err != nil {
		logrus.Error(fmt.Sprintf("配置文件:%sJSON解析失败!", NodeAddrConfigFilePath))
		os.Exit(-1)
	}
	// 初始化节点名称和对应udp端口
	routerNumber = len(nodeAddrs)
	for i, node := range nodeAddrs {
		// 完善映射关系
		NodeToIndex[node.Name] = i
		PortToIndex[node.Port] = i

		routerNames = append(routerNames, node.Name)
		routerPorts = append(routerPorts, node.Port)
		// 完善消息队列
		sendAndSaveMessageQueue = append(sendAndSaveMessageQueue, make(chan Router, MESSAGEQUEUEMAXLENGTH))
	}

	costData, err := os.ReadFile(NodeCostConfigFilePath)
	if err != nil {
		logrus.Error("读取配置文件失败!")
		os.Exit(-1)
	}

	err = json.Unmarshal(costData, &costFromNodeToNodes)
	if err != nil {
		logrus.Error(fmt.Sprintf("配置文件:%sJSON解析失败!", NodeCostConfigFilePath))
		os.Exit(-1)
	}
}
