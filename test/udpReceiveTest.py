import json
import socket
import time


def send_udp_message(message: str, host: str, port: int) -> None:
    try:
        # 创建UDP套接字
        udp_socket: socket.socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

        # 发送消息到指定的主机和端口
        udp_socket.sendto(message.encode(), (host, port))

        # 关闭套接字连接
        udp_socket.close()
    except Exception as e:
        print(f"Error sending UDP message: {e}")


if __name__ == "__main__":
    # 指定目标主机和端口
    target_host: str = "127.0.0.1"  # 可以根据需要修改为实际的目标主机
    target_port: int = 8013  # 可以根据需要修改为实际的目标端口

    try:
        while True:
            # 要发送的消息
            d = {"name": "nodeB", "port": 8012}
            message: str = json.dumps(d)

            # 发送消息
            send_udp_message(message, target_host, target_port)

            # 等待一秒钟
            time.sleep(1)
    except KeyboardInterrupt:
        print("\nUser interrupted the script. Exiting.")
