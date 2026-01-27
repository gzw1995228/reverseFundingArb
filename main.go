package main

import (
	"log"
	"os"
	"time"
)

func main() {
	// 从环境变量获取微信webhook
	webhookURL := os.Getenv("WECHAT_WEBHOOK")
	if webhookURL == "" {
		log.Println("警告: 未配置WECHAT_WEBHOOK环境变量，将无法发送微信通知")
	}

	monitor := NewMonitor(webhookURL, 0.004) // 0.4% 阈值

	// 首次获取所有交易所的资金费率结算周期信息
	log.Println("正在初始化，获取所有交易所的资金费率结算周期...")
	if err := monitor.InitializeExchanges(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	log.Println("初始化完成，开始监控...")
	
	// 每10秒获取一次数据并分析
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 立即执行一次
	monitor.CheckArbitrageOpportunities()

	for range ticker.C {
		monitor.CheckArbitrageOpportunities()
	}
}
