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

-- 3. 仓库表
CREATE TABLE IF NOT EXISTS `tbl_warehouses` (
  `id`         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`       VARCHAR(100) NOT NULL COMMENT '仓库名称',
  `code`       VARCHAR(50) DEFAULT NULL COMMENT '仓库编码',
  `address`    VARCHAR(200) DEFAULT NULL COMMENT '仓库地址',
  `manager_id` INT UNSIGNED DEFAULT NULL COMMENT '管理员ID，关联 tbl_users.id',
  `remark`     VARCHAR(200) DEFAULT NULL COMMENT '备注',
  `status`     INT DEFAULT 1 COMMENT '状态：1 启用 0 禁用',
  `created_at` DATETIME DEFAULT NULL,
  `updated_at` DATETIME DEFAULT NULL,
  `deleted_at` DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tbl_warehouses_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='仓库表';

-- 4. 仓位表
CREATE TABLE IF NOT EXISTS `tbl_locations` (
  `id`            INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `warehouse_id`  INT UNSIGNED NOT NULL COMMENT '所属仓库ID，关联 tbl_warehouses.id',
  `name`          VARCHAR(100) NOT NULL COMMENT '仓位名称',
  `code`          VARCHAR(50) DEFAULT NULL COMMENT '仓位编码',
  `current_stock` INT DEFAULT 0 COMMENT '当前库存',
  `remark`        VARCHAR(200) DEFAULT NULL COMMENT '备注',
  `status`        INT DEFAULT 1 COMMENT '状态：1 启用 0 禁用',
  `created_at`    DATETIME DEFAULT NULL,
  `updated_at`    DATETIME DEFAULT NULL,
  `deleted_at`    DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tbl_locations_warehouse_id` (`warehouse_id`),
  KEY `idx_tbl_locations_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='仓位表';

-- 5. 商品表（一台实物一条记录，SN 码全局唯一）
CREATE TABLE IF NOT EXISTS `tbl_products` (
  `id`           INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`         VARCHAR(100) NOT NULL COMMENT '商品名称',
  `sn`           VARCHAR(50)  NOT NULL COMMENT 'SN 码（自动生成，全局唯一）',
  `sku`          VARCHAR(50)  DEFAULT NULL,
  `spu`          VARCHAR(50)  DEFAULT NULL,
  `category`     VARCHAR(50)  NOT NULL COMMENT '一级分类',
  `sub_category` VARCHAR(100) DEFAULT NULL COMMENT '二级分类（文本）',
  `price`        DECIMAL(10,2) DEFAULT NULL COMMENT '价格（元）',
  `owner_name`   VARCHAR(50)  DEFAULT NULL COMMENT '样品归属人',
  `warehouse_id` INT UNSIGNED NOT NULL COMMENT '所在仓库',
  `location_id`  INT UNSIGNED DEFAULT NULL COMMENT '所在仓位',
  `inbound_date` DATE         NOT NULL COMMENT '入库日期（系统自动）',
  `quantity`     INT          NOT NULL DEFAULT 1 COMMENT '入库数量',
  `status`       INT          NOT NULL COMMENT '1 在库 / 2 已借出 / 0 已出库',
  `image`        VARCHAR(255) DEFAULT NULL COMMENT '商品图片路径',
  `remark`       VARCHAR(500) DEFAULT NULL,
  `created_by`   INT UNSIGNED DEFAULT NULL COMMENT '创建人',
  `created_at`   DATETIME     DEFAULT NULL,
  `updated_at`   DATETIME     DEFAULT NULL,
  `deleted_at`   DATETIME     DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tbl_products_sn` (`sn`),
  KEY `idx_tbl_products_warehouse` (`warehouse_id`),
  KEY `idx_tbl_products_category` (`category`),
  KEY `idx_tbl_products_inbound_date` (`inbound_date`),
  KEY `idx_tbl_products_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品表（一台实物一条记录）';

-- 6. 借用单主表
CREATE TABLE IF NOT EXISTS `tbl_borrow_orders` (
  `id`                 INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `borrow_no`          VARCHAR(50)  NOT NULL COMMENT '借用单号（自动生成，唯一）',
  `borrower_id`        INT UNSIGNED NOT NULL COMMENT '借用人，关联 tbl_users.id',
  `borrower_name`      VARCHAR(50)  NOT NULL COMMENT '借用人展示名（快照）',
  `department`         VARCHAR(50)  DEFAULT NULL COMMENT '借用人部门',
  `borrow_date`        DATETIME     NOT NULL COMMENT '借用时间（系统自动）',
  `days`               INT          NOT NULL COMMENT '借用天数（1~365）',
  `expect_return_date` DATE         NOT NULL COMMENT '预计归还时间 = 借用时间 + 天数',
  `actual_return_date` DATETIME     DEFAULT NULL COMMENT '实际归还时间',
  `status`             INT          NOT NULL COMMENT '1 借用中 / 2 已归还（0 预留已取消）',
  `remark`             VARCHAR(500) DEFAULT NULL,
  `created_at`         DATETIME     DEFAULT NULL,
  `updated_at`         DATETIME     DEFAULT NULL,
  `deleted_at`         DATETIME     DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tbl_borrow_orders_no` (`borrow_no`),
  KEY `idx_tbl_borrow_orders_borrower` (`borrower_id`),
  KEY `idx_tbl_borrow_orders_status` (`status`),
  KEY `idx_tbl_borrow_orders_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='借用单主表';

-- 7. 借用明细表
CREATE TABLE IF NOT EXISTS `tbl_borrow_items` (
  `id`                INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_id`          INT UNSIGNED NOT NULL COMMENT '所属借用单，关联 tbl_borrow_orders.id',
  `product_id`        INT UNSIGNED NOT NULL COMMENT '商品，关联 tbl_products.id',
  `product_name`      VARCHAR(100) NOT NULL COMMENT '商品名称快照',
  `product_sn`        VARCHAR(50)  NOT NULL COMMENT 'SN 快照',
  `quantity`          INT          NOT NULL COMMENT '借用数量',
  `returned_quantity` INT          NOT NULL DEFAULT 0 COMMENT '已归还数量',
  `created_at`        DATETIME     DEFAULT NULL,
  `updated_at`        DATETIME     DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tbl_borrow_items_order` (`order_id`),
  KEY `idx_tbl_borrow_items_product` (`product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='借用明细表';

-- 8. 操作日志表
CREATE TABLE IF NOT EXISTS `tbl_operation_logs` (
  `id`           INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`      INT UNSIGNED DEFAULT NULL COMMENT '操作人，关联 tbl_users.id',
  `user_name`    VARCHAR(50)  DEFAULT NULL COMMENT '操作人展示名（快照）',
  `module`       VARCHAR(20)  NOT NULL COMMENT '模块：商品入库/仓库管理/用户管理/借用管理',
  `action`       VARCHAR(20)  NOT NULL COMMENT '操作类型：创建/编辑/删除/借用/归还',
  `product_name` VARCHAR(200) DEFAULT NULL COMMENT '商品名称（快照，多个以、分隔）',
  `detail`       VARCHAR(500) DEFAULT NULL COMMENT '操作内容',
  `created_at`   DATETIME     DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tbl_operation_logs_user` (`user_id`),
  KEY `idx_tbl_operation_logs_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作日志表';

-- =============================================
-- 存量库增量升级（已有数据库时手动执行；新库由上方建表语句直接包含）
-- 2026-09-21: 操作日志表增加商品名称快照列
-- ALTER TABLE `tbl_operation_logs`
--   ADD COLUMN `product_name` VARCHAR(200) DEFAULT NULL COMMENT '商品名称（快照，多个以、分隔）' AFTER `action`;
-- =============================================
