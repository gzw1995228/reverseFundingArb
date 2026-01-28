package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("警告: 未找到 .env 文件，将使用系统环境变量")
	}

	// 添加测试标志
	testFlag := flag.Bool("test", false, "运行测试模式")
	flag.Parse()

	if *testFlag {
		TestAllExchanges()
		return
	}

	// 从环境变量获取微信webhook
	webhookURL := os.Getenv("WECHAT_WEBHOOK")
	if webhookURL == "" {
		log.Println("警告: 未配置WECHAT_WEBHOOK环境变量，将无法发送微信通知")
	} else {
		log.Printf("已加载微信webhook配置")
	}

	monitor := NewMonitor(webhookURL, 0.02) // 阈值由周期动态决定

	// 首次获取所有交易所的资金费率结算周期信息
	log.Println("正在初始化，获取所有交易所的资金费率结算周期...")
	if err := monitor.InitializeExchanges(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	log.Println("初始化完成，开始监控...")
	
	// 每10秒获取一次数据并分析
	dataTicker := time.NewTicker(10 * time.Second)
	defer dataTicker.Stop()

	// 每小时更新一次资金费率结算周期和合约状态
	intervalTicker := time.NewTicker(1 * time.Hour)
	defer intervalTicker.Stop()

	// 立即执行一次
	monitor.CheckArbitrageOpportunities()

	for {
		select {
		case <-dataTicker.C:
			monitor.CheckArbitrageOpportunities()
		case <-intervalTicker.C:
			log.Println("更新资金费率结算周期和合约状态...")
			monitor.UpdateFundingIntervals()
		}
	}
}
