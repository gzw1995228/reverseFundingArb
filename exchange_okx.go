package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type OKXExchange struct {
	client            *http.Client
	fundingIntervals  map[string]float64 // symbol -> interval in hours
	mu                sync.RWMutex
}

func NewOKXExchange() *OKXExchange {
	return &OKXExchange{
		client:           &http.Client{Timeout: 10 * time.Second},
		fundingIntervals: make(map[string]float64),
	}
}

func (o *OKXExchange) Name() string {
	return "OKX"
}

func (o *OKXExchange) Initialize() error {
	return nil
}

func (o *OKXExchange) UpdateFundingIntervals() error {
	// OKX需要逐个查询合约来获取资金费率间隔
	// 先获取所有合约列表
	url := "https://www.okx.com/api/v5/public/instruments?instType=SWAP"
	
	resp, err := o.client.Get(url)
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
		Data []struct {
			InstID string `json:"instId"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	// 获取资金费率信息来计算间隔
	fundingURL := "https://www.okx.com/api/v5/public/funding-rate?instType=SWAP"
	fundingResp, err := o.client.Get(fundingURL)
	if err != nil {
		return fmt.Errorf("获取资金费率失败: %v", err)
	}
	defer fundingResp.Body.Close()

	fundingBody, err := io.ReadAll(fundingResp.Body)
	if err != nil {
		return fmt.Errorf("读取资金费率响应失败: %v", err)
	}

	var fundingResponse struct {
		Code string `json:"code"`
		Data []struct {
			InstID          string `json:"instId"`
			FundingTime     string `json:"fundingTime"`
			NextFundingTime string `json:"nextFundingTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(fundingBody, &fundingResponse); err != nil {
		return fmt.Errorf("解析资金费率响应失败: %v", err)
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	for _, item := range fundingResponse.Data {
		fundingTime := parseInt64(item.FundingTime)
		nextFundingTime := parseInt64(item.NextFundingTime)
		
		if fundingTime > 0 && nextFundingTime > fundingTime {
			intervalMs := nextFundingTime - fundingTime
			intervalHour := float64(intervalMs) / (1000.0 * 3600.0)
			
			// 转换为统一格式 (BTC-USDT-SWAP -> BTCUSDT)
			if len(item.InstID) > 10 && item.InstID[len(item.InstID)-9:] == "-USDT-SWAP" {
				symbol := item.InstID[:len(item.InstID)-10] + "USDT"
				o.fundingIntervals[symbol] = intervalHour
			}
		}
	}

	return nil
}

func (o *OKXExchange) getFundingInterval(symbol string) float64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	if interval, ok := o.fundingIntervals[symbol]; ok {
		return interval
	}
	return 8.0 // 默认8小时
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
			FundingTime     string `json:"fundingTime"`
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
		
		// 计算资金费率间隔
		fundingTime := parseInt64(item.FundingTime)
		nextFundingTime := parseInt64(item.NextFundingTime)
		intervalHour := 8.0 // 默认
		
		if fundingTime > 0 && nextFundingTime > fundingTime {
			intervalMs := nextFundingTime - fundingTime
			intervalHour = float64(intervalMs) / (1000.0 * 3600.0)
			
			// 更新缓存
			o.mu.Lock()
			o.fundingIntervals[symbol] = intervalHour
			o.mu.Unlock()
		} else {
			intervalHour = o.getFundingInterval(symbol)
		}

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
