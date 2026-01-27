package main

type ContractData struct {
	Symbol              string
	Price               float64
	FundingRate         float64
	FundingIntervalHour float64 // 结算周期（小时）
	FundingRate4h       float64 // 转换为4小时的资金费率
	NextFundingTime     int64
}

type Exchange interface {
	Name() string
	Initialize() error
	FetchFundingRates() (map[string]*ContractData, error)
	UpdateFundingIntervals() error // 更新资金费率结算周期
}
