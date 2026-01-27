package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BinanceExchange struct {
	client *http.Client
}

func NewBinanceExchange() *BinanceExchange {
	return &BinanceExchange{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *BinanceExchange) Name() string {
	return "Binance"
}

func (b *BinanceExchange) Initialize() error {
	// 币安资金费率每8小时结算一次
	return nil
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

		if price > 0 {
			result[item.Symbol] = &ContractData{
				Symbol:          item.Symbol,
				Price:           price,
				FundingRate:     fundingRate,
				FundingInterval: "8h",
				NextFundingTime: item.NextFundingTime,
			}
		}
	}

	return result, nil
}
