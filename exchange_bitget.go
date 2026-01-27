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
	// 获取资金费率和价格信息（使用tickers接口，包含fundingRate）
	url := "https://api.bitget.com/api/v2/mix/market/tickers?productType=USDT-FUTURES"
	
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
			Symbol               string `json:"symbol"`
			LastPr               string `json:"lastPr"`
			MarkPrice            string `json:"markPrice"`
			FundingRate          string `json:"fundingRate"`
			IndexPrice           string `json:"indexPrice"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if response.Code != "00000" {
		return nil, fmt.Errorf("API返回错误: %s - %s", response.Code, response.Msg)
	}

	// 获取资金费率结算周期信息
	fundingURL := "https://api.bitget.com/api/v2/mix/market/current-fund-rate?productType=USDT-FUTURES"
	
	fundingResp, err := b.client.Get(fundingURL)
	if err != nil {
		return nil, fmt.Errorf("获取资金费率信息失败: %v", err)
	}
	defer fundingResp.Body.Close()

	fundingBody, err := io.ReadAll(fundingResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取资金费率响应失败: %v", err)
	}

	var fundingResponse struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol               string `json:"symbol"`
			FundingRateInterval  string `json:"fundingRateInterval"` // 单位：小时
			NextFundingTime      string `json:"nextFundingTime"`     // 下次结算时间戳（毫秒）
		} `json:"data"`
	}

	if err := json.Unmarshal(fundingBody, &fundingResponse); err != nil {
		return nil, fmt.Errorf("解析资金费率响应失败: %v", err)
	}

	if fundingResponse.Code != "00000" {
		return nil, fmt.Errorf("API返回错误: %s - %s", fundingResponse.Code, fundingResponse.Msg)
	}

	// 构建资金费率周期和下次结算时间映射
	fundingIntervalMap := make(map[string]float64)
	nextFundingTimeMap := make(map[string]int64)
	
	for _, item := range fundingResponse.Data {
		intervalHour := parseFloat(item.FundingRateInterval)
		if intervalHour > 0 {
			fundingIntervalMap[item.Symbol] = intervalHour
			
			// 更新缓存
			b.mu.Lock()
			b.fundingIntervals[item.Symbol] = intervalHour
			b.mu.Unlock()
		}
		
		nextFundingTime := parseInt64(item.NextFundingTime)
		if nextFundingTime > 0 {
			nextFundingTimeMap[item.Symbol] = nextFundingTime
		}
	}

	result := make(map[string]*ContractData)
	
	for _, item := range response.Data {
		// 只处理USDT合约（symbol不包含下划线或特殊后缀）
		if len(item.Symbol) < 7 || !isUSDTContract(item.Symbol) {
			continue
		}

		price := parseFloat(item.LastPr)
		if price <= 0 {
			price = parseFloat(item.MarkPrice)
		}
		if price <= 0 {
			price = parseFloat(item.IndexPrice)
		}
		
		if price <= 0 {
			continue
		}

		fundingRate := parseFloat(item.FundingRate)
		
		// 获取结算周期
		intervalHour := fundingIntervalMap[item.Symbol]
		if intervalHour == 0 {
			intervalHour = b.getFundingInterval(item.Symbol)
		}
		
		// 获取下次结算时间
		nextFundingTime := nextFundingTimeMap[item.Symbol]

		result[item.Symbol] = &ContractData{
			Symbol:              item.Symbol,
			Price:               price,
			FundingRate:         fundingRate,
			FundingIntervalHour: intervalHour,
			FundingRate4h:       fundingRate * (4.0 / intervalHour), // 保留用于兼容性
			NextFundingTime:     nextFundingTime,
		}
	}

	return result, nil
}

// isUSDTContract 检查是否是USDT合约
func isUSDTContract(symbol string) bool {
	// Bitget USDT合约通常是 BTCUSDT, ETHUSDT 等格式
	// 不包含下划线或特殊后缀
	if len(symbol) < 7 {
		return false
	}
	
	// 检查是否以USDT结尾
	if len(symbol) >= 4 && symbol[len(symbol)-4:] == "USDT" {
		// 检查是否包含不允许的后缀
		if len(symbol) > 4 && symbol[len(symbol)-5] == '_' {
			return false // 排除 _USDT 格式
		}
		return true
	}
	
	return false
}
