package main

import (
	"DistanceVectorRoutingAlgorithm/router"
	"errors"
	"os"
)

func main() {
	args := os.Args

	// 第一个参数是使用的节点名称
	if len(args) != 2 {
		err := errors.New("输入参数格式错误,应该为路由节点名称")
		panic(err)
	}

	// 初始化
	router.InitConfig()

	node, err := router.New(args[1])
	if err != nil {
		panic(err)
	}

	// 启动路由器
	router.DistanceVectorRoutingAlgorithm(node)
}
