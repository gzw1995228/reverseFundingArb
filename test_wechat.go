// +build ignore

package main

import (
	"fmt"
	"os"
	"time"
)

// 测试微信推送功能
// 运行: go run test_wechat.go
func main() {
	webhookURL := os.Getenv("WECHAT_WEBHOOK")
	if webhookURL == "" {
		fmt.Println("错误: 请先设置 WECHAT_WEBHOOK 环境变量")
		fmt.Println("示例: set WECHAT_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY")
		return
	}

	message := `🔔 测试消息

这是一条测试消息，用于验证企业微信机器人配置是否正确。

如果你收到这条消息，说明配置成功！

时间: ` + fmt.Sprintf("%v", time.Now().Format("2006-01-02 15:04:05"))

	fmt.Println("正在发送测试消息...")
	fmt.Println("Webhook URL:", webhookURL)
	
	if err := SendWechatMessage(webhookURL, message); err != nil {
		fmt.Printf("发送失败: %v\n", err)
		return
	}

	fmt.Println("✓ 测试消息发送成功！请检查企业微信群聊")
}
