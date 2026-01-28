package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type BybitExchange struct {
	client *http.Client
	mu     sync.RWMutex
}

func NewBybitExchange() *BybitExchange {
	return &BybitExchange{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *BybitExchange) Name() string {
	return "Bybit"
}

func (b *BybitExchange) Initialize() error {
	return nil
}

func (b *BybitExchange) UpdateFundingIntervals() error {
	// Bybit的资金费率周期在ticker接口中返回
	return nil
}

func (b *BybitExchange) FetchFundingRates() (map[string]*ContractData, error) {
	url := "https://api.bybit.com/v5/market/tickers?category=linear"
	
	resp, err := b.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var response struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			Category string `json:"category"`
			List     []struct {
				Symbol              string `json:"symbol"`
				LastPrice           string `json:"lastPrice"`
				FundingRate         string `json:"fundingRate"`
				NextFundingTime     string `json:"nextFundingTime"`
				FundingIntervalHour string `json:"fundingIntervalHour"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if response.RetCode != 0 {
		return nil, fmt.Errorf("API返回错误: %s", response.RetMsg)
	}

	result := make(map[string]*ContractData)
	
	for _, item := range response.Result.List {
		// 只处理USDT合约
		if len(item.Symbol) < 4 || item.Symbol[len(item.Symbol)-4:] != "USDT" {
			continue
		}

		price := parseFloat(item.LastPrice)
		if price <= 0 {
			price = parseFloat(item.MarkPrice)
		}
		
		fundingRate := parseFloat(item.FundingRate)
		intervalHour := parseFloat(item.FundingIntervalHour)
		
		if intervalHour == 0 {
			intervalHour = 8.0 // 默认8小时
		}

		if price > 0 {
			// 转换为4小时费率
			fundingRate4h := fundingRate * (4.0 / intervalHour)
			
			result[item.Symbol] = &ContractData{
				Symbol:              item.Symbol,
				Price:               price,
				FundingRate:         fundingRate,
				FundingIntervalHour: intervalHour,
				FundingRate4h:       fundingRate4h,
				NextFundingTime:     parseInt64(item.NextFundingTime),
			}
		}
	}

	return result, nil
}
