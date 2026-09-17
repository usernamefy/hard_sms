package database

import (
	"fmt"
	"log"

	"sms/config"
	"sms/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Init 初始化 MySQL 数据库连接
// 注意：建库建表及默认数据的 SQL 不在代码中执行，
// 请参照 docs/init.sql 手动初始化数据库
func Init() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.App.DBUser, config.App.DBPassword,
		config.App.DBHost, config.App.DBPort, config.App.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: "tbl_", // 所有表统一前缀：users -> tbl_users
		},
	})
	if err != nil {
		log.Fatalf("MySQL 连接失败（请确认 MySQL 已启动、库已创建、账号密码正确，配置见 config/config.go）: %v", err)
	}

	models.SetDB(db)
	log.Printf("MySQL 连接成功: %s:%d/%s", config.App.DBHost, config.App.DBPort, config.App.DBName)
}
