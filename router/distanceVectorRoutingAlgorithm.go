package router

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	mu sync.Mutex
)

// 距离矢量路由算法
func DistanceVectorRoutingAlgorithm(router *Router) {
	var (
		// 锁
		wg sync.WaitGroup

		// 主线程控制协程的阻塞队列
		done chan struct{} = make(chan struct{})

		// 处理路由表的消息队列
		handleMessageQueue chan Router = make(chan Router, MESSAGEQUEUEMAXLENGTH)

		// 处理断连节点的消息队列
		handleDisconnectMessageQueue chan string = make(chan string, MESSAGEQUEUEMAXLENGTH)
	)

	// 开启监听
	router.ReceiveMessage(done, &wg, sendAndSaveMessageQueue)

	// 发送并将消息存储在消息队列中
	sendAndSave := func(port int) {
		defer wg.Done()
		defer close(sendAndSaveMessageQueue[PortToIndex[port]])

		for {
			// 发送消息等待接收，规定时间内收不到视作断连
			router.SendConfiguration(port)
			select {
			case <-done:
				logrus.Info("发送并保存消息协程结束")
				return
			case otherRouter := <-sendAndSaveMessageQueue[PortToIndex[port]]:
				// 收到了响应消息，将消息放入处理路由表的消息队列中等待对应协程处理
				handleMessageQueue <- otherRouter
			case <-time.After(WAITTIME * time.Second):
				// 没有在规定时间内收到消息，视作断连，放入处理断连情况的消息队列中，等待对应协程处理
				handleDisconnectMessageQueue <- routerNames[PortToIndex[port]]
			}
		}
	}

	for p := range PortToIndex {
		if p != router.Port {
			wg.Add(1)
			go sendAndSave(p)
		}
	}

	// 专门处理路由表的协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(handleMessageQueue)

		for {
			select {
			case <-done:
				logrus.Info("处理路由表消息队列的协程结束")
				return
			case otherRouter := <-handleMessageQueue:
				// 写操作上锁
				mu.Lock()
				router.distanceVectorRoutingAlgorithmOnRouter(otherRouter)
				mu.Unlock()
			case <-time.After(HANDLECYCLE * time.Second):
				// 超时不处理跳过
				continue
			}

		}
	}()

	// 专门处理断连的消息队列
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(handleDisconnectMessageQueue)

		for {
			select {
			case <-done:
				logrus.Info("处理断连消息队列的协程结束")
				return
			case otherRouter := <-handleDisconnectMessageQueue:
				// 写操作上锁
				mu.Lock()
				router.HandleDisconnectNode(otherRouter)
				mu.Unlock()
			case <-time.After(HANDLECYCLE * time.Second):
				// 超时不处理跳过
				continue
			}

		}
	}()

	for {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		input := scanner.Text()
		quit := false
		switch input {
		case "router":
			mu.Lock()
			fmt.Println(router.RouterTable())
			mu.Unlock()
		case "quit":
			quit = true
		default:
			continue
		}

		if quit {
			break
		}
	}

	// 结束时先关通道后等待
	defer wg.Wait()
	defer close(done)
}
