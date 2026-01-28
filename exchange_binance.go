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
		Symbol               string `json:"symbol"`
		FundingIntervalHours int    `json:"fundingIntervalHours"`
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
	// 1. 使用 premiumIndex 获取资金费率和下次结算时间
	premiumURL := "https://fapi.binance.com/fapi/v1/premiumIndex"
	
	resp, err := b.client.Get(premiumURL)
	if err != nil {
		return nil, fmt.Errorf("请求premiumIndex失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取premiumIndex响应失败: %v", err)
	}

	var premiumIndexes []struct {
		Symbol          string `json:"symbol"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
	}

	if err := json.Unmarshal(body, &premiumIndexes); err != nil {
		return nil, fmt.Errorf("解析premiumIndex响应失败: %v", err)
	}

	// 2. 使用 /fapi/v1/ticker/24hr 获取价格和24h交易额
	tickerURL := "https://fapi.binance.com/fapi/v1/ticker/24hr"
	
	tickerResp, err := b.client.Get(tickerURL)
	if err != nil {
		return nil, fmt.Errorf("请求ticker/24hr失败: %v", err)
	}
	defer tickerResp.Body.Close()

	tickerBody, err := io.ReadAll(tickerResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取ticker/24hr响应失败: %v", err)
	}

	var tickers []struct {
		Symbol      string `json:"symbol"`
		LastPrice   string `json:"lastPrice"`
		QuoteVolume string `json:"quoteVolume"` // 24h成交额
	}

	if err := json.Unmarshal(tickerBody, &tickers); err != nil {
		return nil, fmt.Errorf("解析ticker/24hr响应失败: %v", err)
	}

	// 构建价格和交易额映射
	type TickerData struct {
		Price       float64
		QuoteVolume float64
	}
	tickerMap := make(map[string]TickerData)
	for _, t := range tickers {
		price := parseFloat(t.LastPrice)
		quoteVolume := parseFloat(t.QuoteVolume)
		if price > 0 {
			tickerMap[t.Symbol] = TickerData{
				Price:       price,
				QuoteVolume: quoteVolume,
			}
		}
	}

	result := make(map[string]*ContractData)
	
	for _, item := range premiumIndexes {
		// 只处理USDT合约
		if len(item.Symbol) < 4 || item.Symbol[len(item.Symbol)-4:] != "USDT" {
			continue
		}

		// 获取价格和交易额
		ticker, ok := tickerMap[item.Symbol]
		if !ok || ticker.Price <= 0 {
			continue
		}
		
		// 过滤24h交易额小于100万的合约
		if ticker.QuoteVolume < 1000000 {
			continue
		}
		
		fundingRate := parseFloat(item.LastFundingRate)
		intervalHour := b.getFundingInterval(item.Symbol)

		// 转换为4小时费率
		fundingRate4h := fundingRate * (4.0 / intervalHour)
		
		result[item.Symbol] = &ContractData{
			Symbol:              item.Symbol,
			Price:               ticker.Price,
			FundingRate:         fundingRate,
			FundingIntervalHour: intervalHour,
			FundingRate4h:       fundingRate4h,
			NextFundingTime:     item.NextFundingTime,
		}
	}

	return result, nil
}
