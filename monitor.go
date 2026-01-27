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
	webhookURL string
	threshold  float64
	exchanges  []Exchange
	mu         sync.RWMutex
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
		},
	}
}

func (m *Monitor) InitializeExchanges() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(m.exchanges))

	for _, exchange := range m.exchanges {
		wg.Add(1)
		go func(ex Exchange) {
			defer wg.Done()
			if err := ex.Initialize(); err != nil {
				errChan <- fmt.Errorf("%s 初始化失败: %v", ex.Name(), err)
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
		log.Printf("%s 获取到 %d 个合约", data.Name, len(data.Contracts))
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

		// 找出最高和最低资金费率
		var rates []struct {
			exchange string
			rate     float64
			price    float64
			contract *ContractData
		}

		for exName, contract := range exchanges {
			if contract.Price <= 0 || math.IsNaN(contract.FundingRate) {
				continue
			}
			rates = append(rates, struct {
				exchange string
				rate     float64
				price    float64
				contract *ContractData
			}{exName, contract.FundingRate, contract.Price, contract})
		}

		if len(rates) < 2 {
			continue
		}

		// 按资金费率排序
		sort.Slice(rates, func(i, j int) bool {
			return rates[i].rate < rates[j].rate
		})

		lowRate := rates[0]
		highRate := rates[len(rates)-1]

		// 计算价差比
		priceSpread := (lowRate.price - highRate.price) / highRate.price

		// 计算净收益
		netProfit := (highRate.rate - lowRate.rate) - priceSpread

		if netProfit > m.threshold {
			opportunities = append(opportunities, ArbitrageOpportunity{
				Symbol:           symbol,
				HighRateExchange: highRate.exchange,
				LowRateExchange:  lowRate.exchange,
				HighRate:         highRate.rate,
				LowRate:          lowRate.rate,
				HighPrice:        highRate.price,
				LowPrice:         lowRate.price,
				PriceSpread:      priceSpread,
				NetProfit:        netProfit,
				HighRatePeriod:   highRate.contract.FundingInterval,
				LowRatePeriod:    lowRate.contract.FundingInterval,
				Timestamp:        time.Now(),
			})
		}
	}

	// 按净收益排序
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].NetProfit > opportunities[j].NetProfit
	})

	return opportunities
}

func (m *Monitor) sendNotifications(opportunities []ArbitrageOpportunity) {
	if m.webhookURL == "" {
		log.Println("未配置微信webhook，跳过通知")
		return
	}

	// 只发送前5个最佳机会
	count := len(opportunities)
	if count > 5 {
		count = 5
	}

	message := fmt.Sprintf("🔔 发现 %d 个套利机会\n\n", len(opportunities))
	
	for i := 0; i < count; i++ {
		opp := opportunities[i]
		message += fmt.Sprintf("【%s】\n", opp.Symbol)
		message += fmt.Sprintf("净收益: %.4f%% (阈值: %.2f%%)\n", opp.NetProfit*100, m.threshold*100)
		message += fmt.Sprintf("高费率: %s %.4f%% (%s)\n", opp.HighRateExchange, opp.HighRate*100, opp.HighRatePeriod)
		message += fmt.Sprintf("低费率: %s %.4f%% (%s)\n", opp.LowRateExchange, opp.LowRate*100, opp.LowRatePeriod)
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
	Symbol           string
	HighRateExchange string
	LowRateExchange  string
	HighRate         float64
	LowRate          float64
	HighPrice        float64
	LowPrice         float64
	PriceSpread      float64
	NetProfit        float64
	HighRatePeriod   string
	LowRatePeriod    string
	Timestamp        time.Time
}
