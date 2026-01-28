package main

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

type Monitor struct {
	webhookURL        string
	threshold         float64
	exchanges         []Exchange
	lastNotifications map[string]time.Time // symbol -> last notification time
	mu                sync.RWMutex
}

func NewMonitor(webhookURL string, threshold float64) *Monitor {
	return &Monitor{
		webhookURL: webhookURL,
		threshold:  threshold,
		exchanges: []Exchange{
			NewBinanceExchange(),
			NewOKXExchange(),
			NewBybitExchange(),
			NewMEXCExchange(),
			NewBitgetExchange(),
			NewGateExchange(),
		},
		lastNotifications: make(map[string]time.Time),
	}
}

func (m *Monitor) InitializeExchanges() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(m.exchanges)*2)

	for _, exchange := range m.exchanges {
		wg.Add(1)
		go func(ex Exchange) {
			defer wg.Done()
			if err := ex.Initialize(); err != nil {
				errChan <- fmt.Errorf("%s 初始化失败: %v", ex.Name(), err)
			} else if err := ex.UpdateFundingIntervals(); err != nil {
				errChan <- fmt.Errorf("%s 更新结算周期失败: %v", ex.Name(), err)
			} else if err := ex.UpdateContractStatus(); err != nil {
				errChan <- fmt.Errorf("%s 更新合约状态失败: %v", ex.Name(), err)
			} else {
				log.Printf("%s 初始化成功", ex.Name())
			}
		}(exchange)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		log.Printf("错误: %v", err)
	}

	return nil
}

func (m *Monitor) UpdateFundingIntervals() {
	log.Println("开始更新所有交易所的结算周期和合约状态...")
	var wg sync.WaitGroup
	for _, exchange := range m.exchanges {
		wg.Add(1)
		go func(ex Exchange) {
			defer wg.Done()
			if err := ex.UpdateFundingIntervals(); err != nil {
				log.Printf("%s 更新结算周期失败: %v", ex.Name(), err)
			} else if err := ex.UpdateContractStatus(); err != nil {
				log.Printf("%s 更新合约状态失败: %v", ex.Name(), err)
			} else {
				log.Printf("%s 结算周期和合约状态更新成功", ex.Name())
			}
		}(exchange)
	}
	wg.Wait()
	log.Println("所有交易所结算周期和合约状态更新完成")
}

func (m *Monitor) CheckArbitrageOpportunities() {
	// 并发获取所有交易所数据
	type ExchangeData struct {
		Name     string
		Contracts map[string]*ContractData
		Error    error
	}

	dataChan := make(chan ExchangeData, len(m.exchanges))
	var wg sync.WaitGroup

	for _, exchange := range m.exchanges {
		wg.Add(1)
		go func(ex Exchange) {
			defer wg.Done()
			contracts, err := ex.FetchFundingRates()
			dataChan <- ExchangeData{
				Name:      ex.Name(),
				Contracts: contracts,
				Error:     err,
			}
		}(exchange)
	}

	wg.Wait()
	close(dataChan)

	// 收集数据
	exchangeDataMap := make(map[string]map[string]*ContractData)
	for data := range dataChan {
		if data.Error != nil {
			log.Printf("%s 获取数据失败: %v", data.Name, data.Error)
			continue
		}
		exchangeDataMap[data.Name] = data.Contracts
	}

	// 分析套利机会
	opportunities := m.analyzeArbitrage(exchangeDataMap)

	// 发送通知
	if len(opportunities) > 0 {
		m.sendNotifications(opportunities)
	}
}

func (m *Monitor) analyzeArbitrage(exchangeData map[string]map[string]*ContractData) []ArbitrageOpportunity {
	// 构建每个币种在各交易所的数据
	symbolMap := make(map[string]map[string]*ContractData)

	for exchangeName, contracts := range exchangeData {
		for symbol, contract := range contracts {
			if symbolMap[symbol] == nil {
				symbolMap[symbol] = make(map[string]*ContractData)
			}
			symbolMap[symbol][exchangeName] = contract
		}
	}

	var opportunities []ArbitrageOpportunity

	// 对每个币种分析
	for symbol, exchanges := range symbolMap {
		if len(exchanges) < 2 {
			continue
		}

		// 收集有效的交易所数据
		var exchangeList []struct {
			name     string
			contract *ContractData
		}

		for exName, contract := range exchanges {
			if contract.Price <= 0 || math.IsNaN(contract.FundingRate) || contract.NextFundingTime <= 0 {
				continue
			}
			exchangeList = append(exchangeList, struct {
				name     string
				contract *ContractData
			}{exName, contract})
		}

		if len(exchangeList) < 2 {
			continue
		}

		// 收集所有不同的下次结算时间戳并排序
		fundingTimestamps := make(map[int64]bool)
		for _, ex := range exchangeList {
			fundingTimestamps[ex.contract.NextFundingTime] = true
		}

		// 转换为切片并排序（从小到大）
		var timestamps []int64
		for ts := range fundingTimestamps {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		// 对每个时间戳分析套利机会
		for _, targetTimestamp := range timestamps {
			opps := m.analyzeAtTimestamp(symbol, exchangeList, targetTimestamp, timestamps)
			opportunities = append(opportunities, opps...)
		}
	}

	// 按净收益排序
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].NetProfit > opportunities[j].NetProfit
	})

	return opportunities
}

// analyzeAtTimestamp 分析在特定时间戳的套利机会
func (m *Monitor) analyzeAtTimestamp(symbol string, exchangeList []struct {
	name     string
	contract *ContractData
}, targetTimestamp int64, allTimestamps []int64) []ArbitrageOpportunity {
	
	var opportunities []ArbitrageOpportunity
	currentTime := time.Now().Unix() * 1000 // 转换为毫秒

	// 计算到目标时间戳的时间差（小时）
	timeToTarget := float64(targetTimestamp-currentTime) / (1000.0 * 3600.0)
	if timeToTarget <= 0 {
		return opportunities // 时间戳已过期
	}

	// 为每个交易所计算在目标时间戳时的累计费率
	type ExchangeRate struct {
		name              string
		price             float64
		originalRate      float64
		accumulatedRate   float64 // 到目标时间的累计费率
		nextFundingTime   int64
		fundingInterval   float64
		settlementsCount  int // 结算次数
	}

	var rates []ExchangeRate

	for _, ex := range exchangeList {
		accumulatedRate := 0.0
		settlementsCount := 0

		if ex.contract.NextFundingTime <= targetTimestamp {
			// 该交易所会在目标时间前结算
			// 计算从现在到目标时间会结算几次
			intervalMs := ex.contract.FundingIntervalHour * 3600.0 * 1000.0
			
			// 计算结算次数
			if intervalMs > 0 {
				// 从下次结算时间到目标时间的时间差
				timeDiff := float64(targetTimestamp - ex.contract.NextFundingTime)
				settlementsCount = 1 + int(timeDiff/intervalMs) // 至少结算一次
				
				// 累计费率 = 单次费率 × 结算次数
				accumulatedRate = ex.contract.FundingRate * float64(settlementsCount)
			}
		} else {
			// 该交易所在目标时间前不会结算，费率为0
			accumulatedRate = 0.0
			settlementsCount = 0
		}

		rates = append(rates, ExchangeRate{
			name:             ex.name,
			price:            ex.contract.Price,
			originalRate:     ex.contract.FundingRate,
			accumulatedRate:  accumulatedRate,
			nextFundingTime:  ex.contract.NextFundingTime,
			fundingInterval:  ex.contract.FundingIntervalHour,
			settlementsCount: settlementsCount,
		})
	}

	// 找出最高和最低累计费率
	if len(rates) < 2 {
		return opportunities
	}

	// 按累计费率排序
	sort.Slice(rates, func(i, j int) bool {
		return rates[i].accumulatedRate < rates[j].accumulatedRate
	})

	lowRate := rates[0]
	highRate := rates[len(rates)-1]

	// 计算价差比
	priceSpread := (lowRate.price - highRate.price) / highRate.price

	// 计算净收益
	netProfit := (highRate.accumulatedRate - lowRate.accumulatedRate) - priceSpread

	// 统一阈值 0.4%
	threshold := m.threshold
	if threshold == 0 {
		threshold = 0.004 // 默认0.4%
	}

	if netProfit > threshold {
		// 格式化目标时间为 UTC+8
		targetTime := time.Unix(targetTimestamp/1000, 0).In(time.FixedZone("CST", 8*3600))
		
		opportunities = append(opportunities, ArbitrageOpportunity{
			Symbol:              symbol,
			HighRateExchange:    highRate.name,
			LowRateExchange:     lowRate.name,
			HighRate:            highRate.originalRate,
			LowRate:             lowRate.originalRate,
			HighPrice:           highRate.price,
			LowPrice:            lowRate.price,
			PriceSpread:         priceSpread,
			NetProfit:           netProfit,
			HighRateIntervalH:   highRate.fundingInterval,
			LowRateIntervalH:    lowRate.fundingInterval,
			TargetTimestamp:     targetTimestamp,
			TargetTime:          targetTime,
			TimeToTarget:        timeToTarget,
			HighAccumulatedRate: highRate.accumulatedRate,
			LowAccumulatedRate:  lowRate.accumulatedRate,
			HighSettlements:     highRate.settlementsCount,
			LowSettlements:      lowRate.settlementsCount,
			Timestamp:           time.Now(),
		})
	}

	return opportunities
}

// getThresholdByInterval 统一阈值为1%
func (m *Monitor) getThresholdByInterval(interval float64) float64 {
	return 0.01 // 1%
}

func (m *Monitor) sendNotifications(opportunities []ArbitrageOpportunity) {
	if m.webhookURL == "" {
		log.Println("未配置微信webhook，跳过通知")
		return
	}

	// 过滤出需要通知的机会（1小时内未通知过的）
	now := time.Now()
	var validOpportunities []ArbitrageOpportunity
	
	m.mu.Lock()
	for _, opp := range opportunities {
		// 生成唯一标识：symbol + 高费率交易所 + 低费率交易所
		key := fmt.Sprintf("%s_%s_%s", opp.Symbol, opp.HighRateExchange, opp.LowRateExchange)
		
		lastTime, exists := m.lastNotifications[key]
		if !exists || now.Sub(lastTime) >= 1*time.Hour {
			validOpportunities = append(validOpportunities, opp)
			m.lastNotifications[key] = now
		}
	}
	m.mu.Unlock()
	
	if len(validOpportunities) == 0 {
		log.Println("所有套利机会在1小时内已通知过，跳过通知")
		return
	}

	// 只发送前5个最佳机会
	count := len(validOpportunities)
	if count > 5 {
		count = 5
	}
	
	// 获取阈值
	threshold := m.threshold
	if threshold == 0 {
		threshold = 0.004 // 默认0.4%
	}

	message := fmt.Sprintf("🔔 发现 %d 个套利机会\n\n", len(validOpportunities))
	
	for i := 0; i < count; i++ {
		opp := validOpportunities[i]
		
		message += fmt.Sprintf("【%s】\n", opp.Symbol)
		message += fmt.Sprintf("目标时间: %s (%.2f小时后)\n", 
			opp.TargetTime.Format("01-02 15:04"), opp.TimeToTarget)
		message += fmt.Sprintf("净收益: %.4f%% (阈值: %.2f%%)\n", opp.NetProfit*100, threshold*100)
		
		// 高费率方
		if opp.HighSettlements > 0 {
			message += fmt.Sprintf("高费率: %s %.4f%% × %d次 = %.4f%%\n", 
				opp.HighRateExchange, opp.HighRate*100, 
				opp.HighSettlements, opp.HighAccumulatedRate*100)
		} else {
			message += fmt.Sprintf("高费率: %s 0%% (未结算)\n", opp.HighRateExchange)
		}
		
		// 低费率方
		if opp.LowSettlements > 0 {
			message += fmt.Sprintf("低费率: %s %.4f%% × %d次 = %.4f%%\n", 
				opp.LowRateExchange, opp.LowRate*100, 
				opp.LowSettlements, opp.LowAccumulatedRate*100)
		} else {
			message += fmt.Sprintf("低费率: %s 0%% (未结算)\n", opp.LowRateExchange)
		}
		
		message += fmt.Sprintf("价差比: %.4f%%\n", opp.PriceSpread*100)
		message += fmt.Sprintf("价格: %.4f / %.4f\n", opp.HighPrice, opp.LowPrice)
		message += "\n"
	}

	if err := SendWechatMessage(m.webhookURL, message); err != nil {
		log.Printf("发送微信通知失败: %v", err)
	} else {
		log.Printf("已发送微信通知，包含 %d 个套利机会", count)
	}
}

type ArbitrageOpportunity struct {
	Symbol              string
	HighRateExchange    string
	LowRateExchange     string
	HighRate            float64   // 原始费率
	LowRate             float64   // 原始费率
	HighPrice           float64
	LowPrice            float64
	PriceSpread         float64
	NetProfit           float64
	HighRateIntervalH   float64   // 结算周期（小时）
	LowRateIntervalH    float64   // 结算周期（小时）
	TargetTimestamp     int64     // 目标结算时间戳（毫秒）
	TargetTime          time.Time // 目标结算时间
	TimeToTarget        float64   // 距离目标时间（小时）
	HighAccumulatedRate float64   // 高费率方累计费率
	LowAccumulatedRate  float64   // 低费率方累计费率
	HighSettlements     int       // 高费率方结算次数
	LowSettlements      int       // 低费率方结算次数
	Timestamp           time.Time
}
