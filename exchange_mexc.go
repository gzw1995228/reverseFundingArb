package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type MEXCExchange struct {
	client            *http.Client
	fundingIntervals  map[string]float64 // symbol -> interval in hours
	mu                sync.RWMutex
}

func NewMEXCExchange() *MEXCExchange {
	return &MEXCExchange{
		client:           &http.Client{Timeout: 10 * time.Second},
		fundingIntervals: make(map[string]float64),
	}
}

func (m *MEXCExchange) Name() string {
	return "MEXC"
}

func (m *MEXCExchange) Initialize() error {
	return nil
}

func (m *MEXCExchange) UpdateFundingIntervals() error {
	// MEXC的资金费率接口包含collectCycle字段
	url := "https://contract.mexc.com/api/v1/contract/funding_rate"
	
	resp, err := m.client.Get(url)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	var response struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    []struct {
			Symbol       string  `json:"symbol"`
			FundingRate  float64 `json:"fundingRate"`
			CollectCycle int     `json:"collectCycle"` // 单位：小时
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if !response.Success {
		return fmt.Errorf("API返回错误，code: %d", response.Code)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range response.Data {
		if item.CollectCycle > 0 {
			// 转换symbol格式
			symbol := item.Symbol
			if len(symbol) > 5 && symbol[len(symbol)-5:] == "_USDT" {
				symbol = symbol[:len(symbol)-5] + "USDT"
			}
			m.fundingIntervals[symbol] = float64(item.CollectCycle)
		}
	}

	return nil
}

func (m *MEXCExchange) getFundingInterval(symbol string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if interval, ok := m.fundingIntervals[symbol]; ok {
		return interval
	}
	return 8.0 // 默认8小时
}

func (m *MEXCExchange) FetchFundingRates() (map[string]*ContractData, error) {
	// MEXC合约API
	url := "https://contract.mexc.com/api/v1/contract/funding_rate"
	
	resp, err := m.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var response struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    []struct {
			Symbol          string  `json:"symbol"`
			FundingRate     float64 `json:"fundingRate"`
			CollectCycle    int     `json:"collectCycle"`    // 单位：小时
			NextSettleTime  int64   `json:"nextSettleTime"`  // 下次结算时间戳（毫秒）
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API返回错误，code: %d", response.Code)
	}

	// 获取价格和交易额信息
	priceURL := "https://contract.mexc.com/api/v1/contract/ticker"
	priceResp, err := m.client.Get(priceURL)
	if err != nil {
		return nil, fmt.Errorf("获取价格失败: %v", err)
	}
	defer priceResp.Body.Close()

	priceBody, err := io.ReadAll(priceResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取价格响应失败: %v", err)
	}

	var priceResponse struct {
		Success bool `json:"success"`
		Data    []struct {
			Symbol    string  `json:"symbol"`
			LastPrice float64 `json:"lastPrice"`
			Amount24  float64 `json:"amount24"` // 24h成交额
		} `json:"data"`
	}

	if err := json.Unmarshal(priceBody, &priceResponse); err != nil {
		return nil, fmt.Errorf("解析价格响应失败: %v", err)
	}

	type TickerData struct {
		Price   float64
		Amount24 float64
	}
	tickerMap := make(map[string]TickerData)
	for _, item := range priceResponse.Data {
		if item.LastPrice > 0 {
			tickerMap[item.Symbol] = TickerData{
				Price:   item.LastPrice,
				Amount24: item.Amount24,
			}
		}
	}

	result := make(map[string]*ContractData)
	
	for _, item := range response.Data {
		// MEXC的symbol格式如 BTC_USDT，转换为 BTCUSDT
		if len(item.Symbol) < 5 {
			continue
		}

		ticker, ok := tickerMap[item.Symbol]
		if !ok || ticker.Price <= 0 {
			continue
		}
		
		// 过滤24h交易额小于100万的合约
		if ticker.Amount24 < 1000000 {
			continue
		}

		// 转换symbol格式
		symbol := item.Symbol
		if len(symbol) > 5 && symbol[len(symbol)-5:] == "_USDT" {
			symbol = symbol[:len(symbol)-5] + "USDT"
		}

		intervalHour := float64(item.CollectCycle)
		if intervalHour == 0 {
			intervalHour = m.getFundingInterval(symbol)
		} else {
			// 更新缓存
			m.mu.Lock()
			m.fundingIntervals[symbol] = intervalHour
			m.mu.Unlock()
		}

		// 转换为4小时费率
		fundingRate4h := item.FundingRate * (4.0 / intervalHour)

		result[symbol] = &ContractData{
			Symbol:              symbol,
			Price:               ticker.Price,
			FundingRate:         item.FundingRate,
			FundingIntervalHour: intervalHour,
			FundingRate4h:       fundingRate4h,
			NextFundingTime:     item.NextSettleTime,
		}
	}

	return result, nil
}
