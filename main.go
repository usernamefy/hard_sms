package main

import (
	"errors"
	"fmt"
	"log"
	"syscall"

	"sms/config"
	"sms/database"
	"sms/routes"
)

func main() {
	// 初始化数据库（自动迁移 + 默认 admin 数据）
	database.Init()

	// 初始化路由
	router := routes.Setup()

	addr := fmt.Sprintf(":%d", config.App.Port)
	log.Printf("库存管理系统启动成功，访问地址: http://localhost%s/login", addr)
	if err := router.Run(addr); err != nil {
		// 端口被占用时给出可操作的提示（10048 = Windows 的 WSAEADDRINUSE）
		if errors.Is(err, syscall.EADDRINUSE) || errors.Is(err, syscall.Errno(10048)) {
			log.Fatalf("启动失败: 端口 %d 已被占用（可能已有服务在运行）。请先停止占用该端口的程序，或在 config/config.go 中修改端口", config.App.Port)
		}
		log.Fatalf("服务启动失败: %v", err)
	}
}
