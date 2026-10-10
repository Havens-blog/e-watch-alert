use watchalert;

-- watchalert_role_binding：eiam_uid → 内部角色（admin/user）桥接表。
-- 与 eiam 共享会话鉴权配套（middleware.Auth() 按 eiam_uid 查本表做角色映射）。
-- 乐观锁 version 列：POST/DELETE 携带 version 时按 WHERE eiam_uid=? AND version=? 更新，不匹配返回 409。
CREATE TABLE IF NOT EXISTS `watchalert_role_binding` (
    `eiam_uid`   BIGINT      NOT NULL COMMENT 'eiam 用户唯一标识',
    `role`       VARCHAR(16) NOT NULL COMMENT 'admin | user；非法值由接口层 400 拒绝',
    `version`    BIGINT      NOT NULL DEFAULT 0 COMMENT '乐观锁版本号，更新时校验并自增',
    `created_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`eiam_uid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
