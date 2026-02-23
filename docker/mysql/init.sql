-- MySQL initialization script
-- Runs once when the container is first created

-- Ensure strict SQL mode for data integrity
SET GLOBAL sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION';

-- Create database if not exists (usually already created by MYSQL_DATABASE env var)
CREATE DATABASE IF NOT EXISTS `trader`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE `trader`;

-- Grant all privileges to the trader user
GRANT ALL PRIVILEGES ON `trader`.* TO 'trader'@'%';
FLUSH PRIVILEGES;
