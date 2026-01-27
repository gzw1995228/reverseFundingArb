# 快速启动指南 ⚡

## 3步开始使用

### 第1步：配置企业微信机器人 🤖

1. 在企业微信群中添加机器人
2. 复制 Webhook 地址
3. 设置环境变量：

**Windows CMD:**
```cmd
set WECHAT_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY
```

**Windows PowerShell:**
```powershell
$env:WECHAT_WEBHOOK="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"
```

### 第2步：测试配置（可选）✅

```cmd
go run test_wechat.go wechat.go
```

如果收到测试消息，说明配置成功！

### 第3步：启动监控 🚀

**方式1：使用脚本（推荐）**
```cmd
run.bat
```

**方式2：直接运行**
```cmd
go run .
```

**方式3：编译后运行**
```cmd
build.bat
funding-rate-monitor.exe
```

## 你会看到什么？

### 控制台输出
```
2026/01/27 10:00:00 正在初始化，获取所有交易所的资金费率结算周期...
2026/01/27 10:00:01 Binance 初始化成功
2026/01/27 10:00:01 OKX 初始化成功
2026/01/27 10:00:01 Bybit 初始化成功
2026/01/27 10:00:01 MEXC 初始化成功
2026/01/27 10:00:01 Bitget 初始化成功
2026/01/27 10:00:01 初始化完成，开始监控...
2026/01/27 10:00:02 Binance 获取到 245 个合约
2026/01/27 10:00:02 OKX 获取到 198 个合约
...
```

### 微信通知（当发现机会时）
```
🔔 发现 3 个套利机会

【BTCUSDT】
净收益: 0.45% (阈值: 0.40%)
高费率: Binance 0.10% (8h)
低费率: OKX -0.05% (8h)
价差比: -0.30%
价格: 45000.0000 / 45100.0000
```

## 常见问题 ❓

### Q: 没有收到微信通知？
A: 检查：
1. WECHAT_WEBHOOK 环境变量是否设置
2. Webhook地址是否正确
3. 是否有套利机会（净收益 > 0.4%）

### Q: 如何修改阈值？
A: 编辑 `main.go` 第17行：
```go
monitor := NewMonitor(webhookURL, 0.005) // 改为 0.5%
```

### Q: 如何修改监控频率？
A: 编辑 `main.go` 第27行：
```go
ticker := time.NewTicker(30 * time.Second) // 改为30秒
```

### Q: 程序占用资源多吗？
A: 非常少！
- 内存: < 50MB
- CPU: < 1%
- 网络: < 100KB/s

## 套利原理简述 📊

1. **找差异**：同一币种在不同交易所的资金费率差异
2. **算收益**：净收益 = 费率差 - 价差
3. **抓机会**：净收益 > 0.4% 时通知

## 下一步 📚

- 📖 详细配置：查看 [CONFIG.md](CONFIG.md)
- 📖 使用说明：查看 [USAGE.md](USAGE.md)
- 📖 项目结构：查看 [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md)

## 支持的交易所 🏦

✅ Binance (币安)
✅ OKX
✅ Bybit
✅ MEXC
✅ Bitget

## 重要提示 ⚠️

- 本程序仅供学习研究使用
- 实际交易需考虑手续费、滑点等成本
- 请遵守各交易所API使用规则
- 交易有风险，投资需谨慎

## 需要帮助？

查看详细文档：
- [README.md](README.md) - 项目概述
- [CONFIG.md](CONFIG.md) - 配置指南
- [USAGE.md](USAGE.md) - 详细使用说明
- [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) - 技术文档

---

祝你套利愉快！💰
