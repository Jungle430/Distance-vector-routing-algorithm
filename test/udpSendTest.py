from typing import Tuple
import socket


def start_udp_server() -> None:
    # 创建UDP socket
    udp_socket: socket.socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

    # 绑定本地地址和端口
    server_address: Tuple[str, int] = ("127.0.0.1", 8013)
    udp_socket.bind(server_address)

    print(f"UDP服务器已启动,监听在 {server_address}")

    try:
        while True:
            # 接收消息
            data, client_address = udp_socket.recvfrom(1024)

            # 打印接收到的消息
            print(f"接收到来自 {client_address} 的消息：{data.decode('utf-8')}")

    except KeyboardInterrupt:
        print("服务器已停止")
    finally:
        # 关闭socket
        udp_socket.close()


if __name__ == "__main__":
    start_udp_server()
