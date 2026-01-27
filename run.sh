#!/bin/bash

# 设置企业微信webhook地址
export WECHAT_WEBHOOK="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY_HERE"

# 运行程序
go run .
