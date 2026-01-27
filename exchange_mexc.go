package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type MEXCExchange struct {
	client *http.Client
}

func NewMEXCExchange() *MEXCExchange {
	return &MEXCExchange{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *MEXCExchange) Name() string {
	return "MEXC"
}

func (m *MEXCExchange) Initialize() error {
	// MEXC资金费率每8小时结算一次
	return nil
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
			Symbol       string  `json:"symbol"`
			FundingRate  float64 `json:"fundingRate"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API返回错误，code: %d", response.Code)
	}

	// 获取价格信息
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
			MarkPrice float64 `json:"fairPrice"`
		} `json:"data"`
	}

	if err := json.Unmarshal(priceBody, &priceResponse); err != nil {
		return nil, fmt.Errorf("解析价格响应失败: %v", err)
	}

	priceMap := make(map[string]float64)
	for _, item := range priceResponse.Data {
		if item.LastPrice > 0 {
			priceMap[item.Symbol] = item.LastPrice
		} else if item.MarkPrice > 0 {
			priceMap[item.Symbol] = item.MarkPrice
		}
	}

	result := make(map[string]*ContractData)
	
	for _, item := range response.Data {
		// MEXC的symbol格式如 BTC_USDT，转换为 BTCUSDT
		if len(item.Symbol) < 5 {
			continue
		}

		price, ok := priceMap[item.Symbol]
		if !ok || price <= 0 {
			continue
		}

		// 转换symbol格式
		symbol := item.Symbol
		if len(symbol) > 5 && symbol[len(symbol)-5:] == "_USDT" {
			symbol = symbol[:len(symbol)-5] + "USDT"
		}

		result[symbol] = &ContractData{
			Symbol:          symbol,
			Price:           price,
			FundingRate:     item.FundingRate,
			FundingInterval: "8h",
			NextFundingTime: 0, // MEXC API可能不提供
		}
	}

	return result, nil
}
