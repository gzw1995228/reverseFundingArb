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
	tradingSymbols    map[string]bool    // symbol -> is trading
	mu                sync.RWMutex
}

func NewBitgetExchange() *BitgetExchange {
	return &BitgetExchange{
		client:           &http.Client{Timeout: 10 * time.Second},
		fundingIntervals: make(map[string]float64),
		tradingSymbols:   make(map[string]bool),
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

func (b *BitgetExchange) isTrading(symbol string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	trading, ok := b.tradingSymbols[symbol]
	return ok && trading
}

func (b *BitgetExchange) UpdateContractStatus() error {
	url := "https://api.bitget.com/api/v2/mix/market/contracts?productType=USDT-FUTURES"
	
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
			Symbol       string `json:"symbol"`
			SymbolStatus string `json:"symbolStatus"`
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
		b.tradingSymbols[item.Symbol] = (item.SymbolStatus == "normal")
	}

	return nil
}

func (b *BitgetExchange) FetchFundingRates() (map[string]*ContractData, error) {
	// 获取资金费率和价格信息（使用tickers接口，包含fundingRate和quoteVolume）
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
			Symbol      string `json:"symbol"`
			LastPr      string `json:"lastPr"`
			FundingRate string `json:"fundingRate"`
			QuoteVolume string `json:"quoteVolume"` // 24h成交额
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
		Code        string `json:"code"`
		Msg         string `json:"msg"`
		RequestTime int64  `json:"requestTime"`
		Data        []struct {
			Symbol              string `json:"symbol"`
			FundingRate         string `json:"fundingRate"`
			FundingRateInterval string `json:"fundingRateInterval"` // 单位：小时
			NextUpdate          string `json:"nextUpdate"`          // 下次更新时间戳（毫秒）
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
		
		nextUpdate := parseInt64(item.NextUpdate)
		if nextUpdate > 0 {
			nextFundingTimeMap[item.Symbol] = nextUpdate
		}
	}

	result := make(map[string]*ContractData)
	
	for _, item := range response.Data {
		// 只处理USDT合约（symbol不包含下划线或特殊后缀）
		if len(item.Symbol) < 7 || !isUSDTContract(item.Symbol) {
			continue
		}
		
		// 检查合约状态
		if !b.isTrading(item.Symbol) {
			continue
		}

		price := parseFloat(item.LastPr)
		if price <= 0 {
			continue
		}
		
		// 过滤24h交易额小于100万的合约
		quoteVolume := parseFloat(item.QuoteVolume)
		if quoteVolume < 1000000 {
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
