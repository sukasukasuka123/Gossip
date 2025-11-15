package HeartBeat

import (
	"fmt"
	"net/http"
	"time"
)

func Heartbeat(endpoint string) {
	ticker := time.NewTicker(10 * time.Second) // 每10秒发送一次心跳
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// 发送心跳请求
			resp, err := http.Post(endpoint, "application/json", nil)
			if err != nil {
				fmt.Println("Error sending heartbeat:", err)
				continue
			}
			defer resp.Body.Close()
		}
	}
}
