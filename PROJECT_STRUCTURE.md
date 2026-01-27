# 项目结构说明

## 文件列表

```
funding-rate-monitor/
├── main.go                 # 程序入口
├── monitor.go              # 监控核心逻辑
├── types.go                # 数据类型定义
├── wechat.go               # 微信推送功能
├── utils.go                # 工具函数
├── exchange_binance.go     # 币安交易所实现
├── exchange_okx.go         # OKX交易所实现
├── exchange_bybit.go       # Bybit交易所实现
├── exchange_mexc.go        # MEXC交易所实现
├── exchange_bitget.go      # Bitget交易所实现
├── test_wechat.go          # 微信推送测试
├── go.mod                  # Go模块定义
├── run.bat                 # Windows运行脚本
├── run.sh                  # Linux/Mac运行脚本
├── build.bat               # Windows编译脚本
├── .gitignore              # Git忽略文件
├── README.md               # 项目说明
├── CONFIG.md               # 配置指南
├── USAGE.md                # 使用说明
└── PROJECT_STRUCTURE.md    # 本文件
```

## 核心模块说明

### 1. main.go
- 程序入口
- 初始化监控器
- 设置定时任务（每10秒）

### 2. monitor.go
- 监控核心逻辑
- 并发获取所有交易所数据
- 分析套利机会
- 发送通知

关键函数：
- `NewMonitor()`: 创建监控器
- `InitializeExchanges()`: 初始化所有交易所
- `CheckArbitrageOpportunities()`: 检查套利机会
- `analyzeArbitrage()`: 分析套利机会
- `sendNotifications()`: 发送通知

### 3. types.go
定义核心数据结构：

```go
type ContractData struct {
    Symbol          string  // 合约名称
    Price           float64 // 价格
    FundingRate     float64 // 资金费率
    FundingInterval string  // 结算周期
    NextFundingTime int64   // 下次结算时间
}

type Exchange interface {
    Name() string
    Initialize() error
    FetchFundingRates() (map[string]*ContractData, error)
}
```

### 4. wechat.go
企业微信推送功能：
- `SendWechatMessage()`: 发送文本消息到企业微信群

### 5. exchange_*.go
各交易所的具体实现：

#### Binance (币安)
- API: `https://fapi.binance.com/fapi/v1/premiumIndex`
- 资金费率周期: 8小时
- Symbol格式: BTCUSDT

#### OKX
- API: `https://www.okx.com/api/v5/public/funding-rate`
- 资金费率周期: 8小时
- Symbol格式: BTC-USDT-SWAP → 转换为 BTCUSDT

#### Bybit
- API: `https://api.bybit.com/v5/market/tickers?category=linear`
- 资金费率周期: 8小时
- Symbol格式: BTCUSDT

#### MEXC
- API: `https://contract.mexc.com/api/v1/contract/funding_rate`
- 资金费率周期: 8小时
- Symbol格式: BTC_USDT → 转换为 BTCUSDT

#### Bitget
- API: `https://api.bitget.com/api/mix/v1/market/tickers?productType=umcbl`
- 资金费率周期: 8小时
- Symbol格式: BTCUSDT_UMCBL → 转换为 BTCUSDT

## 数据流程

```
1. 初始化
   └─> 创建所有交易所实例
       └─> 调用 Initialize() 获取结算周期信息

2. 定时循环（每10秒）
   └─> CheckArbitrageOpportunities()
       ├─> 并发获取所有交易所数据
       │   ├─> Binance.FetchFundingRates()
       │   ├─> OKX.FetchFundingRates()
       │   ├─> Bybit.FetchFundingRates()
       │   ├─> MEXC.FetchFundingRates()
       │   └─> Bitget.FetchFundingRates()
       │
       ├─> analyzeArbitrage()
       │   ├─> 按币种分组
       │   ├─> 找出最高/最低费率
       │   ├─> 计算价差比
       │   ├─> 计算净收益
       │   └─> 筛选超过阈值的机会
       │
       └─> sendNotifications()
           └─> 发送微信通知
```

## 套利计算逻辑

### 公式

```
Price_Spread = (P_low - P_high) / P_high

Net_Profit = (F_high - F_low) - Price_Spread

其中：
- P_low: 低费率交易所的价格
- P_high: 高费率交易所的价格
- F_low: 低费率
- F_high: 高费率
```

### 触发条件

```
Net_Profit > Threshold (默认 0.4%)
```

### 示例

```
交易所A: BTCUSDT 价格=45000, 费率=0.10%
交易所B: BTCUSDT 价格=45100, 费率=-0.05%

Price_Spread = (45100 - 45000) / 45000 = 0.22%
Net_Profit = (0.10% - (-0.05%)) - 0.22% = -0.07%

结果: 不触发（净收益为负）
```

## 并发设计

### 交易所数据获取
使用 goroutine 并发获取所有交易所数据：

```go
var wg sync.WaitGroup
for _, exchange := range m.exchanges {
    wg.Add(1)
    go func(ex Exchange) {
        defer wg.Done()
        contracts, err := ex.FetchFundingRates()
        // 处理数据
    }(exchange)
}
wg.Wait()
```

优点：
- 减少总等待时间
- 提高响应速度
- 各交易所独立，互不影响

## 错误处理

### 网络错误
- 单个交易所失败不影响其他交易所
- 记录错误日志
- 继续处理其他数据

### 数据异常
- 价格 <= 0 的数据会被过滤
- NaN 资金费率会被跳过
- Symbol格式不匹配的会被忽略

### 通知失败
- 记录错误日志
- 不影响下一轮监控

## 扩展点

### 1. 添加新交易所
实现 `Exchange` 接口：
```go
type NewExchange struct {
    client *http.Client
}

func (n *NewExchange) Name() string {
    return "NewExchange"
}

func (n *NewExchange) Initialize() error {
    // 初始化逻辑
    return nil
}

func (n *NewExchange) FetchFundingRates() (map[string]*ContractData, error) {
    // 获取数据逻辑
    return result, nil
}
```

然后在 `monitor.go` 中添加：
```go
exchanges: []Exchange{
    NewBinanceExchange(),
    NewOKXExchange(),
    // ... 其他交易所
    NewNewExchange(), // 添加新交易所
}
```

### 2. 添加数据持久化
在 `monitor.go` 的 `sendNotifications()` 中添加：
```go
// 保存到数据库
for _, opp := range opportunities {
    db.Save(opp)
}
```

### 3. 添加Web API
创建 `api.go`：
```go
func StartAPI(monitor *Monitor) {
    r := gin.Default()
    r.GET("/opportunities", func(c *gin.Context) {
        // 返回当前套利机会
    })
    r.Run(":8080")
}
```

### 4. 添加WebSocket推送
实时推送套利机会到前端：
```go
func (m *Monitor) BroadcastOpportunities(opportunities []ArbitrageOpportunity) {
    // WebSocket广播
}
```

## 性能考虑

### 内存使用
- 每次获取约 1000+ 个合约数据
- 每个 ContractData 约 100 bytes
- 总内存占用 < 1MB

### CPU使用
- 主要是网络I/O等待
- 计算量很小
- CPU占用 < 1%

### 网络带宽
- 每10秒请求5个交易所
- 每个请求约 50-200KB
- 总带宽 < 100KB/s

## 安全考虑

### API密钥
- 当前只使用公开API，无需密钥
- 如需交易功能，需要安全存储密钥

### 环境变量
- WECHAT_WEBHOOK 包含敏感信息
- 不要提交到代码仓库
- 使用 .gitignore 排除 .env 文件

### 数据验证
- 验证价格和费率的合理性
- 防止异常数据导致错误决策

## 测试

### 单元测试
可以添加测试文件：
```go
// monitor_test.go
func TestAnalyzeArbitrage(t *testing.T) {
    // 测试套利分析逻辑
}
```

### 集成测试
```bash
# 测试微信推送
go run test_wechat.go wechat.go

# 测试完整流程
go run .
```

## 部署建议

### 开发环境
```bash
go run .
```

### 生产环境
```bash
# 编译
go build -o funding-rate-monitor .

# 后台运行
nohup ./funding-rate-monitor > output.log 2>&1 &
```

### Docker部署
可以创建 Dockerfile：
```dockerfile
FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o funding-rate-monitor .
CMD ["./funding-rate-monitor"]
```

## 维护建议

### 日志管理
- 定期清理日志文件
- 使用日志轮转

### 监控告警
- 监控程序运行状态
- API请求失败率
- 通知发送成功率

### 定期更新
- 检查交易所API变更
- 更新依赖包
- 优化算法
