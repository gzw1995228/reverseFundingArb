# 配置指南

## 企业微信机器人配置

### 1. 创建企业微信机器人

1. 在企业微信群聊中，点击右上角 `...` -> `添加群机器人`
2. 选择 `新创建一个机器人`
3. 设置机器人名称（如：资金费率监控）
4. 复制 Webhook 地址，格式如：
   ```
   https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   ```

### 2. 配置环境变量

#### Windows CMD
```cmd
set WECHAT_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY
```

#### Windows PowerShell
```powershell
$env:WECHAT_WEBHOOK="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"
```

#### Linux/Mac
```bash
export WECHAT_WEBHOOK="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"
```

### 3. 永久配置（可选）

#### Windows
在系统环境变量中添加：
1. 右键 `此电脑` -> `属性` -> `高级系统设置`
2. 点击 `环境变量`
3. 在用户变量中新建：
   - 变量名：`WECHAT_WEBHOOK`
   - 变量值：你的webhook地址

#### Linux/Mac
在 `~/.bashrc` 或 `~/.zshrc` 中添加：
```bash
export WECHAT_WEBHOOK="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"
```

然后执行：
```bash
source ~/.bashrc  # 或 source ~/.zshrc
```

## 修改监控阈值

编辑 `main.go` 文件，修改第17行：

```go
monitor := NewMonitor(webhookURL, 0.004) // 0.4% 阈值
```

将 `0.004` 改为你想要的阈值（如 0.005 表示 0.5%）

## 修改监控频率

编辑 `main.go` 文件，修改第27行：

```go
ticker := time.NewTicker(10 * time.Second) // 每10秒
```

将 `10` 改为你想要的秒数

## 运行方式

### 方式1：使用脚本运行（推荐）

Windows:
```cmd
run.bat
```

Linux/Mac:
```bash
chmod +x run.sh
./run.sh
```

### 方式2：直接运行

```bash
go run .
```

### 方式3：编译后运行

Windows:
```cmd
build.bat
funding-rate-monitor.exe
```

Linux/Mac:
```bash
go build -o funding-rate-monitor .
./funding-rate-monitor
```

## 后台运行（Linux/Mac）

使用 nohup：
```bash
nohup ./funding-rate-monitor > output.log 2>&1 &
```

使用 screen：
```bash
screen -S funding-monitor
./funding-rate-monitor
# 按 Ctrl+A 然后按 D 退出screen
```

查看运行中的screen：
```bash
screen -ls
```

重新连接：
```bash
screen -r funding-monitor
```

## Windows服务方式运行

可以使用 NSSM (Non-Sucking Service Manager) 将程序注册为Windows服务：

1. 下载 NSSM: https://nssm.cc/download
2. 安装服务：
   ```cmd
   nssm install FundingRateMonitor "D:\path\to\funding-rate-monitor.exe"
   nssm set FundingRateMonitor AppEnvironmentExtra WECHAT_WEBHOOK=your_webhook_url
   nssm start FundingRateMonitor
   ```

## 故障排查

### 问题1：无法连接到交易所API
- 检查网络连接
- 某些地区可能需要使用代理
- 检查防火墙设置

### 问题2：未收到微信通知
- 确认 WECHAT_WEBHOOK 环境变量已正确设置
- 检查webhook地址是否正确
- 查看程序日志中是否有错误信息

### 问题3：编译失败
- 确保已安装 Go 1.21 或更高版本
- 运行 `go mod tidy` 下载依赖
- 检查网络连接（下载依赖需要访问 Go 模块代理）

## 日志说明

程序运行时会输出以下日志：

- `正在初始化...` - 程序启动
- `Binance 初始化成功` - 交易所连接成功
- `Binance 获取到 X 个合约` - 成功获取数据
- `发现 X 个套利机会` - 发现套利机会
- `已发送微信通知` - 通知发送成功
