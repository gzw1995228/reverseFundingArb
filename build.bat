@echo off
echo 正在编译程序...
go build -o funding-rate-monitor.exe .
if %errorlevel% == 0 (
    echo 编译成功！可执行文件: funding-rate-monitor.exe
) else (
    echo 编译失败！
)
pause
