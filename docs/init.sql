-- =============================================
-- 库存管理系统 数据库初始化脚本（MySQL 版）
-- 执行方式:
--   命令行: mysql -u root -p < docs/init.sql
--   图形工具: Navicat 等连接 MySQL 后粘贴执行（库名 sms）
-- =============================================

-- 0. 创建数据库（已存在则跳过）
CREATE DATABASE IF NOT EXISTS `sms` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
USE `sms`;

-- 1. 用户表
CREATE TABLE IF NOT EXISTS `tbl_users` (
  `id`         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username`   VARCHAR(50) NOT NULL,
  `password`   VARCHAR(32) NOT NULL,
  `real_name`  VARCHAR(50) DEFAULT NULL,
  `role`       VARCHAR(20) DEFAULT 'admin',
  `status`     INT DEFAULT 1,
  `last_login` DATETIME DEFAULT NULL,
  `created_at` DATETIME DEFAULT NULL,
  `updated_at` DATETIME DEFAULT NULL,
  `deleted_at` DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tbl_users_username` (`username`),
  KEY `idx_tbl_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 2. 默认管理员：admin / admin123
--    密码密文 = MD5('admin123') = 0192023a7bbd73250516f069df18b500
INSERT INTO `tbl_users`
  (`username`, `password`, `real_name`, `role`, `status`, `created_at`, `updated_at`)
VALUES
  ('admin', '0192023a7bbd73250516f069df18b500', '系统管理员', 'admin', 1, NOW(), NOW());
