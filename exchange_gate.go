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
	mu                sync.RWMutex
}

func NewGateExchange() *GateExchange {
	return &GateExchange{
		client:           &http.Client{Timeout: 10 * time.Second},
		fundingIntervals: make(map[string]float64),
	}
}

func (g *GateExchange) Name() string {
	return "Gate"
}

func (g *GateExchange) Initialize() error {
	return nil
}

func (g *GateExchange) UpdateFundingIntervals() error {
	// Gate.io的合约信息接口包含funding_interval字段（单位：秒）
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
		Name            string `json:"name"`
		FundingInterval int    `json:"funding_interval"` // 单位：秒
	}

	if err := json.Unmarshal(body, &contracts); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	for _, contract := range contracts {
		if contract.FundingInterval > 0 {
			intervalHour := float64(contract.FundingInterval) / 3600.0
			// Gate的symbol格式如 BTC_USDT，转换为 BTCUSDT
			symbol := contract.Name
			if len(symbol) > 5 && symbol[len(symbol)-5:] == "_USDT" {
				symbol = symbol[:len(symbol)-5] + "USDT"
			}
			g.fundingIntervals[symbol] = intervalHour
		}
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
		Contract     string `json:"contract"`
		Last         string `json:"last"`
		MarkPrice    string `json:"mark_price"`
		FundingRate  string `json:"funding_rate"`
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

		price := parseFloat(ticker.Last)
		if price <= 0 {
			price = parseFloat(ticker.MarkPrice)
		}
		
		fundingRate := parseFloat(ticker.FundingRate)

		if price > 0 {
			// 转换symbol格式
			symbol := ticker.Contract
			if len(symbol) > 5 && symbol[len(symbol)-5:] == "_USDT" {
				symbol = symbol[:len(symbol)-5] + "USDT"
			}

			intervalHour := g.getFundingInterval(symbol)
			
			// 转换为4小时费率
			fundingRate4h := fundingRate * (4.0 / intervalHour)

			result[symbol] = &ContractData{
				Symbol:              symbol,
				Price:               price,
				FundingRate:         fundingRate,
				FundingIntervalHour: intervalHour,
				FundingRate4h:       fundingRate4h,
				NextFundingTime:     0,
			}
		}
	}

	return result, nil
}
