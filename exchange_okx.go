package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OKXExchange struct {
	client *http.Client
}

func NewOKXExchange() *OKXExchange {
	return &OKXExchange{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (o *OKXExchange) Name() string {
	return "OKX"
}

func (o *OKXExchange) Initialize() error {
	// OKX资金费率每8小时结算一次
	return nil
}

func (o *OKXExchange) FetchFundingRates() (map[string]*ContractData, error) {
	url := "https://www.okx.com/api/v5/public/funding-rate?instType=SWAP"
	
	resp, err := o.client.Get(url)
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
		Data []struct {
			InstID          string `json:"instId"`
			FundingRate     string `json:"fundingRate"`
			NextFundingRate string `json:"nextFundingRate"`
			NextFundingTime string `json:"nextFundingTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if response.Code != "0" {
		return nil, fmt.Errorf("API返回错误: %s", response.Code)
	}

	// 获取价格信息
	priceURL := "https://www.okx.com/api/v5/market/tickers?instType=SWAP"
	priceResp, err := o.client.Get(priceURL)
	if err != nil {
		return nil, fmt.Errorf("获取价格失败: %v", err)
	}
	defer priceResp.Body.Close()

	priceBody, err := io.ReadAll(priceResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取价格响应失败: %v", err)
	}

	var priceResponse struct {
		Code string `json:"code"`
		Data []struct {
			InstID   string `json:"instId"`
			Last     string `json:"last"`
			MarkPx   string `json:"markPx"`
		} `json:"data"`
	}

	if err := json.Unmarshal(priceBody, &priceResponse); err != nil {
		return nil, fmt.Errorf("解析价格响应失败: %v", err)
	}

	priceMap := make(map[string]float64)
	for _, item := range priceResponse.Data {
		if price := parseFloat(item.Last); price > 0 {
			priceMap[item.InstID] = price
		} else if price := parseFloat(item.MarkPx); price > 0 {
			priceMap[item.InstID] = price
		}
	}

	result := make(map[string]*ContractData)
	
	for _, item := range response.Data {
		// 只处理USDT合约
		if len(item.InstID) < 9 || item.InstID[len(item.InstID)-9:] != "-USDT-SWAP" {
			continue
		}

		fundingRate := parseFloat(item.FundingRate)
		price, ok := priceMap[item.InstID]
		if !ok || price <= 0 {
			continue
		}

		// 转换为统一格式 (BTC-USDT-SWAP -> BTCUSDT)
		symbol := item.InstID[:len(item.InstID)-10] + "USDT"

		result[symbol] = &ContractData{
			Symbol:          symbol,
			Price:           price,
			FundingRate:     fundingRate,
			FundingInterval: "8h",
			NextFundingTime: parseInt64(item.NextFundingTime),
		}
	}

	return result, nil
}
