# 资金费率套利监控系统

监控币安、OKX、Bybit、MEXC、Bitget五大交易所的USDT合约资金费率，自动发现套利机会并通过企业微信推送通知。

## 功能特点

- 支持5大主流交易所：Binance、OKX、Bybit、MEXC、Bitget
- 实时监控所有USDT合约的资金费率和价格
- 自动计算净收益（考虑资金费率差和价差）
- 当净收益超过阈值（默认0.4%）时自动推送微信通知
- 每10秒更新一次数据

## 安装依赖

```bash
go mod download
```

## 配置

设置企业微信机器人webhook地址：

### Windows (CMD)
```cmd
set WECHAT_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY
```

### Windows (PowerShell)
```powershell
$env:WECHAT_WEBHOOK="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"
```

### Linux/Mac
```bash
export WECHAT_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY
```

## 运行

```bash
go run .
```

## 套利逻辑

1. **资金费率差异**：找出同一币种在不同交易所的最高和最低资金费率
2. **价差计算**：Price_Spread = (低费率交易所价格 - 高费率交易所价格) / 高费率交易所价格
3. **净收益**：Net_Profit = (高费率 - 低费率) - 价差比
4. **触发条件**：当 Net_Profit > 0.4% 时发送通知

## 注意事项

- 资金费率通常每8小时结算一次
- 实际套利需要考虑交易手续费、滑点、资金转移成本等
- 建议在实际操作前进行充分的风险评估
- API请求频率请遵守各交易所的限制规则

## 微信通知格式

```
🔔 发现 X 个套利机会

【BTCUSDT】
净收益: 0.45% (阈值: 0.40%)
高费率: Binance 0.10% (8h)
低费率: OKX -0.05% (8h)
价差比: -0.30%
价格: 45000.00 / 45100.00
```

## 自定义阈值

修改 `main.go` 中的阈值参数：

```go
monitor := NewMonitor(webhookURL, 0.004) // 0.4% 阈值
```
