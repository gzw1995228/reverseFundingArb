package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type BitgetExchange struct {
	client            *http.Client
	fundingIntervals  map[string]float64 // symbol -> interval in hours
	mu                sync.RWMutex
}

func NewBitgetExchange() *BitgetExchange {
	return &BitgetExchange{
		client:           &http.Client{Timeout: 10 * time.Second},
		fundingIntervals: make(map[string]float64),
	}
}

func (b *BitgetExchange) Name() string {
	return "Bitget"
}

func (b *BitgetExchange) Initialize() error {
	return nil
}

func (b *BitgetExchange) UpdateFundingIntervals() error {
	// Bitget使用新的API获取资金费率信息
	url := "https://api.bitget.com/api/v2/mix/market/current-fund-rate?productType=USDT-FUTURES"
	
	resp, err := b.client.Get(url)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol               string `json:"symbol"`
			FundingRate          string `json:"fundingRate"`
			FundingRateInterval  string `json:"fundingRateInterval"` // 单位：小时，如 "8"
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if response.Code != "00000" {
		return fmt.Errorf("API返回错误: %s - %s", response.Code, response.Msg)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, item := range response.Data {
		intervalHour := parseFloat(item.FundingRateInterval)
		if intervalHour > 0 {
			// Bitget的symbol格式如 BTCUSDT
			b.fundingIntervals[item.Symbol] = intervalHour
		}
	}

	return nil
}

func (b *BitgetExchange) getFundingInterval(symbol string) float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	if interval, ok := b.fundingIntervals[symbol]; ok {
		return interval
	}
	return 8.0 // 默认8小时
}

func (b *BitgetExchange) FetchFundingRates() (map[string]*ContractData, error) {
	// 获取资金费率
	fundingURL := "https://api.bitget.com/api/v2/mix/market/current-fund-rate?productType=USDT-FUTURES"
	
	fundingResp, err := b.client.Get(fundingURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer fundingResp.Body.Close()

	fundingBody, err := io.ReadAll(fundingResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var fundingResponse struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol               string `json:"symbol"`
			FundingRate          string `json:"fundingRate"`
			FundingRateInterval  string `json:"fundingRateInterval"` // 单位：小时
		} `json:"data"`
	}

	if err := json.Unmarshal(fundingBody, &fundingResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if fundingResponse.Code != "00000" {
		return nil, fmt.Errorf("API返回错误: %s - %s", fundingResponse.Code, fundingResponse.Msg)
	}

	// 获取价格信息
	url := "https://api.bitget.com/api/mix/v1/market/tickers?productType=umcbl"
	
	resp, err := b.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("获取价格失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取价格响应失败: %v", err)
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol      string `json:"symbol"`
			Last        string `json:"last"`
			MarkPrice   string `json:"markPrice"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析价格响应失败: %v", err)
	}

	if response.Code != "00000" {
		return nil, fmt.Errorf("API返回错误: %s - %s", response.Code, response.Msg)
	}

	// 构建价格映射
	priceMap := make(map[string]float64)
	for _, item := range response.Data {
		// Bitget的symbol格式如 BTCUSDT_UMCBL，转换为 BTCUSDT
		if len(item.Symbol) < 11 || item.Symbol[len(item.Symbol)-6:] != "_UMCBL" {
			continue
		}
		
		symbol := item.Symbol[:len(item.Symbol)-6]
		price := parseFloat(item.Last)
		if price <= 0 {
			price = parseFloat(item.MarkPrice)
		}
		if price > 0 {
			priceMap[symbol] = price
		}
	}

	// 构建资金费率映射
	fundingMap := make(map[string]struct {
		rate     float64
		interval float64
	})
	
	for _, item := range fundingResponse.Data {
		fundingRate := parseFloat(item.FundingRate)
		intervalHour := parseFloat(item.FundingRateInterval)
		
		if intervalHour == 0 {
			intervalHour = b.getFundingInterval(item.Symbol)
		} else {
			// 更新缓存
			b.mu.Lock()
			b.fundingIntervals[item.Symbol] = intervalHour
			b.mu.Unlock()
		}
		
		fundingMap[item.Symbol] = struct {
			rate     float64
			interval float64
		}{fundingRate, intervalHour}
	}

	result := make(map[string]*ContractData)
	
	for symbol, price := range priceMap {
		funding, ok := fundingMap[symbol]
		if !ok {
			continue
		}

		// 转换为4小时费率
		fundingRate4h := funding.rate * (4.0 / funding.interval)

		result[symbol] = &ContractData{
			Symbol:              symbol,
			Price:               price,
			FundingRate:         funding.rate,
			FundingIntervalHour: funding.interval,
			FundingRate4h:       fundingRate4h,
			NextFundingTime:     0,
		}
	}

	return result, nil
}
