package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type GateExchange struct {
	client            *http.Client
	fundingIntervals  map[string]float64 // symbol -> interval in hours
	nextFundingTimes  map[string]int64   // symbol -> next funding time (milliseconds)
	tradingSymbols    map[string]bool    // symbol -> is trading
	mu                sync.RWMutex
}

func NewGateExchange() *GateExchange {
	return &GateExchange{
		client:           &http.Client{Timeout: 10 * time.Second},
		fundingIntervals: make(map[string]float64),
		nextFundingTimes: make(map[string]int64),
		tradingSymbols:   make(map[string]bool),
	}
}

func (g *GateExchange) Name() string {
	return "Gate"
}

func (g *GateExchange) Initialize() error {
	return nil
}

func (g *GateExchange) UpdateFundingIntervals() error {
	// Gate.io的合约信息接口包含funding_interval和funding_next_apply字段
	url := "https://api.gateio.ws/api/v4/futures/usdt/contracts"
	
	resp, err := g.client.Get(url)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	var contracts []struct {
		Name              string `json:"name"`
		FundingInterval   int    `json:"funding_interval"`   // 单位：秒
		FundingNextApply  int64  `json:"funding_next_apply"` // 下次结算时间戳（秒）
		InDelisting       bool   `json:"in_delisting"`
		Status            string `json:"status"`
	}

	if err := json.Unmarshal(body, &contracts); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	for _, contract := range contracts {
		// Gate的symbol格式如 BTC_USDT，转换为 BTCUSDT
		symbol := contract.Name
		if len(symbol) > 5 && symbol[len(symbol)-5:] == "_USDT" {
			symbol = symbol[:len(symbol)-5] + "USDT"
		}
		
		if contract.FundingInterval > 0 {
			intervalHour := float64(contract.FundingInterval) / 3600.0
			g.fundingIntervals[symbol] = intervalHour
		}
		
		if contract.FundingNextApply > 0 {
			// 转换为毫秒
			g.nextFundingTimes[symbol] = contract.FundingNextApply * 1000
		}
		
		// 更新合约状态
		g.tradingSymbols[symbol] = (contract.Status == "trading" && !contract.InDelisting)
	}

	return nil
}

func (g *GateExchange) getFundingInterval(symbol string) float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if interval, ok := g.fundingIntervals[symbol]; ok {
		return interval
	}
	return 8.0 // 默认8小时
}

func (g *GateExchange) getNextFundingTime(symbol string) int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	if nextTime, ok := g.nextFundingTimes[symbol]; ok {
		return nextTime
	}
	return 0
}

func (g *GateExchange) isTrading(symbol string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	trading, ok := g.tradingSymbols[symbol]
	return ok && trading
}

func (g *GateExchange) UpdateContractStatus() error {
	// UpdateFundingIntervals 已经获取了合约状态，这里不需要重复
	return nil
}

func (g *GateExchange) FetchFundingRates() (map[string]*ContractData, error) {
	// 获取所有合约的ticker信息
	url := "https://api.gateio.ws/api/v4/futures/usdt/tickers"
	
	resp, err := g.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var tickers []struct {
		Contract        string `json:"contract"`
		Last            string `json:"last"`
		FundingRate     string `json:"funding_rate"`
		Volume24hQuote  string `json:"volume_24h_quote"` // 24h成交额（报价货币）
	}

	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	result := make(map[string]*ContractData)
	
	for _, ticker := range tickers {
		// Gate的symbol格式如 BTC_USDT，转换为 BTCUSDT
		if len(ticker.Contract) < 5 {
			continue
		}

		// 转换symbol格式
		symbol := ticker.Contract
		if len(symbol) > 5 && symbol[len(symbol)-5:] == "_USDT" {
			symbol = symbol[:len(symbol)-5] + "USDT"
		}
		
		// 检查合约状态
		if !g.isTrading(symbol) {
			continue
		}

		price := parseFloat(ticker.Last)
		if price <= 0 {
			continue
		}
		
		// 过滤24h交易额小于100万的合约
		volume24hQuote := parseFloat(ticker.Volume24hQuote)
		if volume24hQuote < 1000000 {
			continue
		}
		
		fundingRate := parseFloat(ticker.FundingRate)

		intervalHour := g.getFundingInterval(symbol)
		nextFundingTime := g.getNextFundingTime(symbol)
		
		// 转换为4小时费率
		fundingRate4h := fundingRate * (4.0 / intervalHour)

		result[symbol] = &ContractData{
			Symbol:              symbol,
			Price:               price,
			FundingRate:         fundingRate,
			FundingIntervalHour: intervalHour,
			FundingRate4h:       fundingRate4h,
			NextFundingTime:     nextFundingTime,
		}
	}

	return result, nil
}
