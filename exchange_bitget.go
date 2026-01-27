package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BitgetExchange struct {
	client *http.Client
}

func NewBitgetExchange() *BitgetExchange {
	return &BitgetExchange{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *BitgetExchange) Name() string {
	return "Bitget"
}

func (b *BitgetExchange) Initialize() error {
	// Bitget资金费率每8小时结算一次
	return nil
}

func (b *BitgetExchange) FetchFundingRates() (map[string]*ContractData, error) {
	// 获取所有合约信息
	url := "https://api.bitget.com/api/mix/v1/market/tickers?productType=umcbl"
	
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
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol      string `json:"symbol"`
			Last        string `json:"last"`
			MarkPrice   string `json:"markPrice"`
			FundingRate string `json:"fundingRate"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if response.Code != "00000" {
		return nil, fmt.Errorf("API返回错误: %s - %s", response.Code, response.Msg)
	}

	result := make(map[string]*ContractData)
	
	for _, item := range response.Data {
		// Bitget的symbol格式如 BTCUSDT_UMCBL，转换为 BTCUSDT
		if len(item.Symbol) < 11 || item.Symbol[len(item.Symbol)-6:] != "_UMCBL" {
			continue
		}

		price := parseFloat(item.Last)
		if price <= 0 {
			price = parseFloat(item.MarkPrice)
		}
		
		fundingRate := parseFloat(item.FundingRate)

		if price > 0 {
			symbol := item.Symbol[:len(item.Symbol)-6]
			
			result[symbol] = &ContractData{
				Symbol:          symbol,
				Price:           price,
				FundingRate:     fundingRate,
				FundingInterval: "8h",
				NextFundingTime: 0, // Bitget API可能不直接提供
			}
		}
	}

	return result, nil
}
