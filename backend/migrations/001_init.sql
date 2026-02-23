-- Migration 001: Initial schema
-- This is the reference SQL schema. GORM AutoMigrate handles actual migrations.
-- Keep this file in sync with models.go for documentation purposes.

SET FOREIGN_KEY_CHECKS = 0;

-- ── symbols ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `symbols` (
  `id`          BIGINT AUTO_INCREMENT PRIMARY KEY,
  `ticker`      VARCHAR(20)  NOT NULL,
  `market`      VARCHAR(20)  NOT NULL,
  `base_asset`  VARCHAR(20)  NOT NULL,
  `quote_asset` VARCHAR(20)  DEFAULT NULL,
  `description` VARCHAR(255) DEFAULT NULL,
  `active`      TINYINT(1)   NOT NULL DEFAULT 1,
  `metadata`    JSON         DEFAULT NULL,
  `created_at`  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`  DATETIME(3)  DEFAULT NULL,
  UNIQUE KEY `idx_symbol_market` (`ticker`, `market`),
  KEY `idx_symbols_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── candles ───────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `candles` (
  `id`        BIGINT AUTO_INCREMENT PRIMARY KEY,
  `symbol`    VARCHAR(20)    NOT NULL,
  `market`    VARCHAR(20)    NOT NULL,
  `timeframe` VARCHAR(10)    NOT NULL,
  `timestamp` DATETIME(3)    NOT NULL,
  `open`      DECIMAL(20,8)  NOT NULL,
  `high`      DECIMAL(20,8)  NOT NULL,
  `low`       DECIMAL(20,8)  NOT NULL,
  `close`     DECIMAL(20,8)  NOT NULL,
  `volume`    DECIMAL(30,8)  NOT NULL,
  `created_at` DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3)   DEFAULT NULL,
  -- Composite index for the primary query pattern
  KEY `idx_candle_lookup` (`symbol`, `market`, `timeframe`, `timestamp`),
  KEY `idx_candles_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── strategy_defs ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `strategy_defs` (
  `id`           BIGINT AUTO_INCREMENT PRIMARY KEY,
  `name`         VARCHAR(100) NOT NULL,
  `description`  TEXT         DEFAULT NULL,
  `params_schema` JSON        DEFAULT NULL,
  `active`       TINYINT(1)   NOT NULL DEFAULT 1,
  `created_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY `idx_strategy_defs_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── backtests ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `backtests` (
  `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
  `name`          VARCHAR(200) NOT NULL,
  `strategy_name` VARCHAR(100) NOT NULL,
  `symbol`        VARCHAR(20)  NOT NULL,
  `market`        VARCHAR(20)  NOT NULL,
  `timeframe`     VARCHAR(10)  NOT NULL,
  `start_date`    DATETIME(3)  NOT NULL,
  `end_date`      DATETIME(3)  NOT NULL,
  `params`        JSON         DEFAULT NULL,
  `status`        VARCHAR(20)  NOT NULL DEFAULT 'pending',
  `metrics`       JSON         DEFAULT NULL,
  `equity_curve`  JSON         DEFAULT NULL,
  `error_message` TEXT         DEFAULT NULL,
  `started_at`    DATETIME(3)  DEFAULT NULL,
  `completed_at`  DATETIME(3)  DEFAULT NULL,
  `created_at`    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`    DATETIME(3)  DEFAULT NULL,
  KEY `idx_backtests_strategy_name` (`strategy_name`),
  KEY `idx_backtests_status` (`status`),
  KEY `idx_backtests_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── trades ────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `trades` (
  `id`           BIGINT AUTO_INCREMENT PRIMARY KEY,
  `backtest_id`  BIGINT        DEFAULT NULL,
  `portfolio_id` BIGINT        DEFAULT NULL,
  `symbol`       VARCHAR(20)   NOT NULL,
  `market`       VARCHAR(20)   NOT NULL,
  `direction`    VARCHAR(10)   NOT NULL,
  `entry_price`  DECIMAL(20,8) NOT NULL,
  `exit_price`   DECIMAL(20,8) DEFAULT NULL,
  `quantity`     DECIMAL(20,8) NOT NULL,
  `entry_time`   DATETIME(3)   NOT NULL,
  `exit_time`    DATETIME(3)   DEFAULT NULL,
  `pnl`          DECIMAL(20,8) DEFAULT NULL,
  `pnl_percent`  DECIMAL(10,4) DEFAULT NULL,
  `fee`          DECIMAL(20,8) NOT NULL DEFAULT 0,
  `metadata`     JSON          DEFAULT NULL,
  `created_at`   DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY `idx_trades_backtest_id`  (`backtest_id`),
  KEY `idx_trades_portfolio_id` (`portfolio_id`),
  KEY `idx_trades_symbol`       (`symbol`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── portfolios ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `portfolios` (
  `id`            BIGINT AUTO_INCREMENT PRIMARY KEY,
  `name`          VARCHAR(200)  NOT NULL,
  `strategy_name` VARCHAR(100)  NOT NULL,
  `symbol`        VARCHAR(20)   NOT NULL,
  `market`        VARCHAR(20)   NOT NULL,
  `timeframe`     VARCHAR(10)   NOT NULL,
  `params`        JSON          DEFAULT NULL,
  `is_live`       TINYINT(1)    NOT NULL DEFAULT 0,
  `is_active`     TINYINT(1)    NOT NULL DEFAULT 1,
  `initial_cash`  DECIMAL(20,8) NOT NULL,
  `current_cash`  DECIMAL(20,8) DEFAULT NULL,
  `current_value` DECIMAL(20,8) DEFAULT NULL,
  `state`         JSON          DEFAULT NULL,
  `created_at`    DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`    DATETIME(3)   DEFAULT NULL,
  KEY `idx_portfolios_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── alerts ────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `alerts` (
  `id`           BIGINT AUTO_INCREMENT PRIMARY KEY,
  `name`         VARCHAR(200) NOT NULL,
  `symbol`       VARCHAR(20)  NOT NULL,
  `market`       VARCHAR(20)  NOT NULL,
  `condition`    VARCHAR(30)  NOT NULL,
  `threshold`    DECIMAL(20,8) NOT NULL,
  `status`       VARCHAR(20)  NOT NULL DEFAULT 'active',
  `message`      TEXT         DEFAULT NULL,
  `triggered_at` DATETIME(3)  DEFAULT NULL,
  `created_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY `idx_alerts_symbol` (`symbol`),
  KEY `idx_alerts_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── notifications ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `notifications` (
  `id`         BIGINT AUTO_INCREMENT PRIMARY KEY,
  `type`       VARCHAR(30)  NOT NULL,
  `title`      VARCHAR(255) NOT NULL,
  `body`       TEXT         DEFAULT NULL,
  `read`       TINYINT(1)   NOT NULL DEFAULT 0,
  `metadata`   JSON         DEFAULT NULL,
  `created_at` DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY `idx_notifications_type`       (`type`),
  KEY `idx_notifications_read`       (`read`),
  KEY `idx_notifications_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── watch_lists ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS `watch_lists` (
  `id`         BIGINT AUTO_INCREMENT PRIMARY KEY,
  `name`       VARCHAR(100) NOT NULL,
  `symbols`    JSON         DEFAULT NULL,
  `created_at` DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3)  DEFAULT NULL,
  KEY `idx_watch_lists_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;
