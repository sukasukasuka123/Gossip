package Logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

type LogStruct struct{}

func NewLogger() *LogStruct {
	return &LogStruct{}
}

// Log 方法实现了你指定的签名，每次调用都会打开、写入并关闭目标文件。
func (LogStruct) Log(content string, fileName string) {
	// 1. 打开或创建文件，使用 O_APPEND 模式确保日志内容追加到末尾
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// 如果无法打开日志文件，则退回到标准错误输出进行警告
		log.Printf("[ERROR] Failed to open log file %s: %v", fileName, err)
		return
	}
	defer file.Close() // 2. 确保文件句柄在函数退出时被关闭

	// 3. 格式化日志内容: 添加时间戳
	timestamp := time.Now().Format("2006/01/02 15:04:05.000000")

	// 4. 将格式化后的内容写入文件，并添加换行符
	_, err = fmt.Fprintf(file, "%s		|		 %s\n", timestamp, content)
	fmt.Printf("%s		|		 %s\n", timestamp, content)
	if err != nil {
		log.Printf("[ERROR] Failed to write to log file %s: %v", fileName, err)
	}
}
