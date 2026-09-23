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
  `id`            INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username`      VARCHAR(50)  NOT NULL,
  `password`      VARCHAR(32)  NOT NULL,
  `real_name`     VARCHAR(50)  DEFAULT NULL,
  `department_id` INT UNSIGNED DEFAULT NULL COMMENT '部门，关联 tbl_departments.id',
  `role`          VARCHAR(20)  DEFAULT 'admin',
  `status`        INT DEFAULT 1,
  `last_login`    DATETIME DEFAULT NULL,
  `created_at`    DATETIME DEFAULT NULL,
  `updated_at`    DATETIME DEFAULT NULL,
  `deleted_at`    DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tbl_users_username` (`username`),
  KEY `idx_tbl_users_department` (`department_id`),
  KEY `idx_tbl_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 2. 默认管理员：admin / admin123
--    密码密文 = MD5('admin123') = 0192023a7bbd73250516f069df18b500
INSERT INTO `tbl_users`
  (`username`, `password`, `real_name`, `role`, `status`, `created_at`, `updated_at`)
VALUES
  ('admin', '0192023a7bbd73250516f069df18b500', '系统管理员', 'admin', 1, NOW(), NOW());

-- 2.1 部门表 + 初始部门数据（技术部 / 销售部 / 运营部）
CREATE TABLE IF NOT EXISTS `tbl_departments` (
  `id`         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`       VARCHAR(50)  NOT NULL COMMENT '部门名称',
  `status`     INT DEFAULT 1 COMMENT '状态：1 启用 0 禁用',
  `created_at` DATETIME DEFAULT NULL,
  `updated_at` DATETIME DEFAULT NULL,
  `deleted_at` DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tbl_departments_name` (`name`),
  KEY `idx_tbl_departments_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='部门表';

INSERT IGNORE INTO `tbl_departments` (`name`, `status`, `created_at`, `updated_at`)
VALUES
  ('技术部', 1, NOW(), NOW()),
  ('销售部', 1, NOW(), NOW()),
  ('运营部', 1, NOW(), NOW());

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
  `owner_id`     INT UNSIGNED DEFAULT NULL COMMENT '样品归属人，关联 tbl_users.id',
  `warehouse_id` INT UNSIGNED NOT NULL COMMENT '所在仓库',
  `location_id`  INT UNSIGNED DEFAULT NULL COMMENT '所在仓位',
  `inbound_date` DATE         NOT NULL COMMENT '入库日期（系统自动）',
  `quantity`     INT          NOT NULL DEFAULT 1 COMMENT '入库数量（当前库存口径）',
  `in_stock_quantity` INT     NOT NULL DEFAULT 0 COMMENT '在库数量（借用扣减/归还回补）',
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

-- 8. 归还单表（一次归还操作一条记录）
CREATE TABLE IF NOT EXISTS `tbl_return_orders` (
  `id`                  INT UNSIGNED  NOT NULL AUTO_INCREMENT,
  `return_no`           VARCHAR(50)   NOT NULL COMMENT '归还单号（自动生成 RT+日期+序号，唯一）',
  `borrow_order_id`     INT UNSIGNED  NOT NULL COMMENT '借用单，关联 tbl_borrow_orders.id',
  `borrow_item_id`      INT UNSIGNED  NOT NULL COMMENT '借用明细，关联 tbl_borrow_items.id',
  `product_id`          INT UNSIGNED  NOT NULL COMMENT '商品，关联 tbl_products.id',
  `product_name`        VARCHAR(100)  NOT NULL COMMENT '商品名称快照',
  `product_sn`          VARCHAR(50)   NOT NULL COMMENT 'SN 快照',
  `quantity`            INT           NOT NULL COMMENT '归还数量',
  `is_lost`             INT           NOT NULL DEFAULT 0 COMMENT '是否商品丢失：1 是 0 否',
  `compensation_amount` DECIMAL(10,2) DEFAULT NULL COMMENT '赔偿金额（丢失时填写）',
  `remark`              VARCHAR(500)  DEFAULT NULL,
  `returned_by_id`      INT UNSIGNED  DEFAULT NULL COMMENT '归还操作人，关联 tbl_users.id',
  `returned_by_name`    VARCHAR(50)   DEFAULT NULL COMMENT '归还操作人展示名（快照）',
  `created_at`          DATETIME      DEFAULT NULL COMMENT '归还时间',
  `updated_at`          DATETIME      DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tbl_return_orders_no` (`return_no`),
  KEY `idx_tbl_return_orders_borrow` (`borrow_order_id`),
  KEY `idx_tbl_return_orders_item` (`borrow_item_id`),
  KEY `idx_tbl_return_orders_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='归还单表';

-- 9. 操作日志表
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

-- 10. 消耗记录表（一次消耗操作一条记录）
CREATE TABLE IF NOT EXISTS `tbl_consume_records` (
  `id`                  INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `consume_no`          VARCHAR(50)  NOT NULL COMMENT '消耗单号（自动生成 CO+日期+序号，唯一）',
  `product_id`          INT UNSIGNED NOT NULL COMMENT '商品，关联 tbl_products.id',
  `product_name`        VARCHAR(100) NOT NULL COMMENT '商品名称快照',
  `product_sn`          VARCHAR(50)  NOT NULL COMMENT 'SN 快照',
  `quantity`            INT          NOT NULL COMMENT '消耗数量',
  `reason`              VARCHAR(50)  DEFAULT NULL COMMENT '消耗原因',
  `remark`              VARCHAR(500) DEFAULT NULL,
  `consumer_id`         INT UNSIGNED DEFAULT NULL COMMENT '消耗人，关联 tbl_users.id',
  `consumer_name`       VARCHAR(50)  DEFAULT NULL COMMENT '消耗人展示名（快照）',
  `consumer_department` VARCHAR(50)  DEFAULT NULL COMMENT '消耗人部门（快照）',
  `created_at`          DATETIME     DEFAULT NULL COMMENT '消耗时间',
  `updated_at`          DATETIME     DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tbl_consume_records_no` (`consume_no`),
  KEY `idx_tbl_consume_records_product` (`product_id`),
  KEY `idx_tbl_consume_records_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消耗记录表';

-- 11. 样品异动记录表（每个样品一条异动记录）
CREATE TABLE IF NOT EXISTS `tbl_transfer_records` (
  `id`                  INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `transfer_no`         VARCHAR(50)  NOT NULL COMMENT '异动单号（自动生成 TR+日期+序号，唯一）',
  `type`                INT          NOT NULL COMMENT '异动类型：1 人员转移 / 2 移仓',
  `product_id`          INT UNSIGNED NOT NULL COMMENT '商品，关联 tbl_products.id',
  `product_name`        VARCHAR(100) NOT NULL COMMENT '商品名称快照',
  `product_sn`          VARCHAR(50)  NOT NULL COMMENT 'SN 快照',
  `from_owner_id`       INT UNSIGNED DEFAULT NULL COMMENT '原归属人，关联 tbl_users.id（人员转移用）',
  `from_owner_name`     VARCHAR(50)  DEFAULT NULL COMMENT '原归属人展示名（快照）',
  `from_department`     VARCHAR(50)  DEFAULT NULL COMMENT '原归属部门（快照）',
  `to_owner_id`         INT UNSIGNED DEFAULT NULL COMMENT '新归属人，关联 tbl_users.id（人员转移用）',
  `to_owner_name`       VARCHAR(50)  DEFAULT NULL COMMENT '新归属人展示名（快照）',
  `to_department`       VARCHAR(50)  DEFAULT NULL COMMENT '新归属部门（快照）',
  `from_warehouse_id`   INT UNSIGNED DEFAULT NULL COMMENT '源仓库（移仓用）',
  `from_warehouse_name` VARCHAR(100) DEFAULT NULL COMMENT '源仓库名称（快照）',
  `from_location_id`    INT UNSIGNED DEFAULT NULL COMMENT '源仓位（0 表示原无仓位）',
  `from_location_name`  VARCHAR(100) DEFAULT NULL COMMENT '源仓库名称（快照）',
  `to_warehouse_id`     INT UNSIGNED DEFAULT NULL COMMENT '目标仓库（移仓用）',
  `to_warehouse_name`   VARCHAR(100) DEFAULT NULL COMMENT '目标仓库名称（快照）',
  `to_location_id`      INT UNSIGNED DEFAULT NULL COMMENT '目标仓位（0 表示无仓位）',
  `to_location_name`    VARCHAR(100) DEFAULT NULL COMMENT '目标仓位名称（快照）',
  `reason`              VARCHAR(50)  DEFAULT NULL COMMENT '异动原因',
  `remark`              VARCHAR(500) DEFAULT NULL,
  `operator_id`         INT UNSIGNED DEFAULT NULL COMMENT '操作人，关联 tbl_users.id',
  `operator_name`       VARCHAR(50)  DEFAULT NULL COMMENT '操作人展示名（快照）',
  `created_at`          DATETIME     DEFAULT NULL COMMENT '异动时间',
  `updated_at`          DATETIME     DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tbl_transfer_records_no` (`transfer_no`),
  KEY `idx_tbl_transfer_records_product` (`product_id`),
  KEY `idx_tbl_transfer_records_type` (`type`),
  KEY `idx_tbl_transfer_records_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='样品异动记录表';

-- =============================================
-- 存量库增量升级（已有数据库时手动执行；新库由上方建表语句直接包含）
-- 2026-09-21: 操作日志表增加商品名称快照列
-- ALTER TABLE `tbl_operation_logs`
--   ADD COLUMN `product_name` VARCHAR(200) DEFAULT NULL COMMENT '商品名称（快照，多个以、分隔）' AFTER `action`;
-- 2026-09-22: 商品表增加在库数量列（借用扣减/归还回补），存量数据回填：在库 = 入库数量 - 借用中未归还
-- ALTER TABLE `tbl_products`
--   ADD COLUMN `in_stock_quantity` INT NOT NULL DEFAULT 0 COMMENT '在库数量（借用扣减/归还回补）' AFTER `quantity`;
-- UPDATE `tbl_products` p
--   SET p.`in_stock_quantity` = GREATEST(p.`quantity` - COALESCE((
--         SELECT SUM(i.`quantity` - i.`returned_quantity`)
--         FROM `tbl_borrow_items` i
--         JOIN `tbl_borrow_orders` o ON o.`id` = i.`order_id` AND o.`deleted_at` IS NULL
--         WHERE o.`status` = 1 AND i.`product_id` = p.`id`), 0), 0)
--   WHERE p.`deleted_at` IS NULL;
-- 2026-09-22: 新增部门表（技术部/销售部/运营部），用户表增加部门列
-- CREATE TABLE IF NOT EXISTS `tbl_departments` ( ... 见上方 2.1 节 ... );
-- ALTER TABLE `tbl_users`
--   ADD COLUMN `department_id` INT UNSIGNED DEFAULT NULL COMMENT '部门，关联 tbl_departments.id' AFTER `real_name`,
--   ADD KEY `idx_tbl_users_department` (`department_id`);
-- 2026-09-22: 商品归属人改为存用户 ID（原 owner_name / owner_department 快照列删除，部门由归属人用户实时带出）
-- ALTER TABLE `tbl_products`
--   ADD COLUMN `owner_id` INT UNSIGNED DEFAULT NULL COMMENT '样品归属人，关联 tbl_users.id' AFTER `owner_department`;
-- UPDATE `tbl_products` p
--   JOIN `tbl_users` u ON u.`real_name` = p.`owner_name` AND u.`deleted_at` IS NULL
--   SET p.`owner_id` = u.`id` WHERE p.`owner_id` IS NULL;
-- UPDATE `tbl_products` p
--   JOIN `tbl_users` u ON u.`username` = p.`owner_name` AND u.`deleted_at` IS NULL
--   SET p.`owner_id` = u.`id` WHERE p.`owner_id` IS NULL;
-- ALTER TABLE `tbl_products` DROP COLUMN `owner_name`, DROP COLUMN `owner_department`;
-- 2026-09-23: 新增消耗记录表（新库由上方建表语句直接包含）
-- CREATE TABLE IF NOT EXISTS `tbl_consume_records` ( ... 见上方第 9 节 ... );
-- =============================================
