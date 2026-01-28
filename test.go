package main

import (
	"fmt"
	"log"
	"sort"
	"time"
)

// TestAllExchanges 测试所有交易所
func TestAllExchanges() {
	fmt.Println("\n开始测试所有交易所...")
	fmt.Println("=" + string(make([]byte, 119)))

	exchanges := []Exchange{
		NewBinanceExchange(),
		NewOKXExchange(),
		NewBybitExchange(),
		NewMEXCExchange(),
		NewBitgetExchange(),
		NewGateExchange(),
	}

	for _, exchange := range exchanges {
		fmt.Printf("\n========== 测试 %s ==========\n", exchange.Name())
		testExchange(exchange)
	}

	fmt.Println("\n" + "=" + string(make([]byte, 119)))
	fmt.Println("所有交易所测试完成")
}

func testExchange(exchange Exchange) {
	// 1. 初始化
	fmt.Printf("1. 初始化 %s...\n", exchange.Name())
	if err := exchange.Initialize(); err != nil {
		log.Printf("   ❌ 初始化失败: %v\n", err)
		return
	}
	fmt.Printf("   ✓ 初始化成功\n")

	// 2. 更新资金费率结算周期
	fmt.Printf("2. 更新资金费率结算周期...\n")
	if err := exchange.UpdateFundingIntervals(); err != nil {
		log.Printf("   ❌ 更新失败: %v\n", err)
		return
	}
	fmt.Printf("   ✓ 更新成功\n")

	// 3. 获取资金费率
	fmt.Printf("3. 获取资金费率和合约价格...\n")
	contracts, err := exchange.FetchFundingRates()
	if err != nil {
		log.Printf("   ❌ 获取失败: %v\n", err)
		return
	}
	fmt.Printf("   ✓ 获取成功，共 %d 个合约\n", len(contracts))

	// 4. 排序并打印前10个合约
	if len(contracts) == 0 {
		fmt.Printf("   ⚠ 没有获取到合约数据\n")
		return
	}

	// 将合约转换为切片并排序
	type ContractInfo struct {
		Symbol string
		Data   *ContractData
	}

	var contractList []ContractInfo
	for symbol, data := range contracts {
		contractList = append(contractList, ContractInfo{symbol, data})
	}

	// 按符号排序
	sort.Slice(contractList, func(i, j int) bool {
		return contractList[i].Symbol < contractList[j].Symbol
	})

	// 打印前10个
	count := len(contractList)
	if count > 10 {
		count = 10
	}

	fmt.Printf("\n4. 前 %d 个合约详情:\n", count)
	fmt.Printf("%-12s | %-10s | %-12s | %-15s | %-20s | %-22s | %-15s\n",
		"合约", "价格", "资金费率", "结算周期(h)", "下次结算时间", "下次结算时间戳", "4h费率")
	fmt.Println("=" + string(make([]byte, 140)))

	for i := 0; i < count; i++ {
		contract := contractList[i]
		data := contract.Data

		// 格式化下次结算时间
		nextFundingTime := time.Unix(data.NextFundingTime/1000, 0).Format("01-02 15:04:05")

		fmt.Printf("%-12s | %10.8f | %12.6f%% | %15.2f | %20s | %22d | %15.6f%%\n",
			contract.Symbol,
			data.Price,
			data.FundingRate*100,
			data.FundingIntervalHour,
			nextFundingTime,
			data.NextFundingTime,
			data.FundingRate4h*100,
		)
	}

	fmt.Printf("\n✓ %s 测试完成\n", exchange.Name())
}
