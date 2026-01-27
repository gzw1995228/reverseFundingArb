package main

type ContractData struct {
	Symbol          string
	Price           float64
	FundingRate     float64
	FundingInterval string // 结算周期，如 "8h", "4h"
	NextFundingTime int64
}

type Exchange interface {
	Name() string
	Initialize() error
	FetchFundingRates() (map[string]*ContractData, error)
}
