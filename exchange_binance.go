package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type BinanceExchange struct {
	client            *http.Client
	fundingIntervals  map[string]float64 // symbol -> interval in hours
	mu                sync.RWMutex
}

func NewBinanceExchange() *BinanceExchange {
	return &BinanceExchange{
		client:           &http.Client{Timeout: 10 * time.Second},
		fundingIntervals: make(map[string]float64),
	}
}

func (b *BinanceExchange) Name() string {
	return "Binance"
}

func (b *BinanceExchange) Initialize() error {
	return nil
}

func (b *BinanceExchange) UpdateFundingIntervals() error {
	url := "https://fapi.binance.com/fapi/v1/fundingInfo"
	
	resp, err := b.client.Get(url)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	var fundingInfos []struct {
		Symbol                   string `json:"symbol"`
		AdjustedFundingRateCap   string `json:"adjustedFundingRateCap"`
		AdjustedFundingRateFloor string `json:"adjustedFundingRateFloor"`
		FundingIntervalHours     int    `json:"fundingIntervalHours"`
	}

	if err := json.Unmarshal(body, &fundingInfos); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	
	for _, info := range fundingInfos {
		if info.FundingIntervalHours > 0 {
			b.fundingIntervals[info.Symbol] = float64(info.FundingIntervalHours)
		}
	}

	return nil
}

func (b *BinanceExchange) getFundingInterval(symbol string) float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	if interval, ok := b.fundingIntervals[symbol]; ok {
		return interval
	}
	return 8.0 // 默认8小时
}

func (b *BinanceExchange) FetchFundingRates() (map[string]*ContractData, error) {
	// 获取所有合约的资金费率
	url := "https://fapi.binance.com/fapi/v1/premiumIndex"
	
	resp, err := b.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var premiumIndexes []struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
	}

	if err := json.Unmarshal(body, &premiumIndexes); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	result := make(map[string]*ContractData)
	
	for _, item := range premiumIndexes {
		// 只处理USDT合约
		if len(item.Symbol) < 4 || item.Symbol[len(item.Symbol)-4:] != "USDT" {
			continue
		}

		price := parseFloat(item.MarkPrice)
		fundingRate := parseFloat(item.LastFundingRate)
		intervalHour := b.getFundingInterval(item.Symbol)

		if price > 0 {
			// 转换为4小时费率
			fundingRate4h := fundingRate * (4.0 / intervalHour)
			
			result[item.Symbol] = &ContractData{
				Symbol:              item.Symbol,
				Price:               price,
				FundingRate:         fundingRate,
				FundingIntervalHour: intervalHour,
				FundingRate4h:       fundingRate4h,
				NextFundingTime:     item.NextFundingTime,
			}
		}
	}

	return result, nil
}
