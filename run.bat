@echo off
REM 设置企业微信webhook地址
set WECHAT_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY_HERE

REM 运行程序
go run .

pause
