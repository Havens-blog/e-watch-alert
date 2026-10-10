use watchalert;

-- 首条 admin 注入脚本（解 bootstrap 鸡生蛋）。
-- 背景：POST /api/watchalert/role-bindings 受 middleware.Auth() 保护，绑定表为空时任何用户 403，
--       无法经 HTTP 创建首条 admin；故首条 admin 须经本脚本（或迁移导入 CLI --bootstrap-admin=<eiam_uid>）注入。
-- 用法：将占位 <eiam_uid> 替换为实际 eiam 用户 uid 后执行。迁移步骤：建表 -> seed 首条 admin -> 启动服务。
INSERT INTO `watchalert_role_binding` (`eiam_uid`, `role`)
VALUES (<eiam_uid>, 'admin')
ON DUPLICATE KEY UPDATE `role` = VALUES(`role`);
