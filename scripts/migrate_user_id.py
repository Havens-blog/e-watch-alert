# -*- coding: utf-8 -*-
"""WatchAlert 租户映射纠正 + 用户 ID 迁移脚本。

1. 租户 tenant_id 纠正：2 (default-tenant) -> 3 (jlc，嘉立创，业务数据实际归属)
2. 用户 user_id 迁移：本地 "admin" -> eiam uid "1"，"dd8cigql"(Haven) -> eiam uid "11"
   含 JSON 字段（duty_schedules.users / duty_managements.manager / duty_calendar_infos.user_group）
   内嵌的 "userid" 值替换。

幂等：重复执行不产生副作用。
"""
import json
import pymysql

DB = dict(host="106.52.187.69", port=3306, user="root", password="123456",
          database="watchalert", charset="utf8mb4")

OLD_TENANT = "2"
NEW_TENANT = "3"  # eiam jlc 租户 id

USER_MAP = {
    "admin": "1",       # eiam admin uid
    "dd8cigql": "11",   # eiam Haven uid
}

TENANT_ID_TABLES = [
    "alert_cur_events", "alert_data_sources", "alert_his_events", "alert_notices",
    "alert_rules", "alert_silences", "audit_logs", "comments", "dashboard_folders",
    "dashboards", "duty_calendar_infos", "duty_managements", "duty_schedules",
    "notice_records", "rule_groups", "w8t_fault_center", "w8t_probe_rule",
    "w8t_prometheus_target", "w8t_prometheus_target_group", "w8t_prometheus_target_version",
    "w8t_recording_rule_groups", "w8t_recording_rules",
]


def replace_userid_in_json(raw, mapping):
    """递归替换 JSON 结构里所有 "userid" 字段的值（按映射表）。"""
    if isinstance(raw, dict):
        return {k: (mapping.get(v, v) if k == "userid" else replace_userid_in_json(v, mapping))
                for k, v in raw.items()}
    if isinstance(raw, list):
        return [replace_userid_in_json(x, mapping) for x in raw]
    return raw


def main():
    conn = pymysql.connect(**DB)
    cur = conn.cursor()
    total = 0

    print("=" * 64)
    print("迁移 1：租户 tenant_id 纠正  2 -> 3 (jlc)")
    print("=" * 64)

    # tenants / tenant_linked_users 表（已废弃，但保持一致）
    for t in ["tenants", "tenant_linked_users"]:
        cur.execute(f"SELECT COUNT(*) FROM `{t}` WHERE id=%s", (OLD_TENANT,))
        cnt = cur.fetchone()[0]
        if cnt > 0:
            cur.execute(f"UPDATE `{t}` SET id=%s WHERE id=%s", (NEW_TENANT, OLD_TENANT))
            total += cnt
            print(f"[{t}] id 2->3: {cnt} 行")

    # 业务表 tenant_id
    for t in TENANT_ID_TABLES:
        try:
            cur.execute(f"SELECT COUNT(*) FROM `{t}` WHERE tenant_id=%s", (OLD_TENANT,))
            cnt = cur.fetchone()[0]
            if cnt > 0:
                cur.execute(f"UPDATE `{t}` SET tenant_id=%s WHERE tenant_id=%s", (NEW_TENANT, OLD_TENANT))
                total += cnt
                print(f"[{t}] tenant_id 2->3: {cnt} 行")
        except Exception as e:
            print(f"[{t}] 跳过: {e}")

    # members.tenants JSON
    cur.execute("SELECT user_id, tenants FROM members WHERE tenants IS NOT NULL")
    for user_id, tenants_json in cur.fetchall():
        try:
            arr = json.loads(tenants_json)
        except (TypeError, json.JSONDecodeError):
            continue
        if not isinstance(arr, list):
            continue
        new_arr = [NEW_TENANT if x == OLD_TENANT else x for x in arr]
        if new_arr != arr:
            cur.execute("UPDATE members SET tenants=%s WHERE user_id=%s",
                        (json.dumps(new_arr, ensure_ascii=False), user_id))
            total += 1
            print(f"[members.tenants] {user_id}: {arr} -> {new_arr}")

    print()
    print("=" * 64)
    print("迁移 2：用户 user_id 迁移  admin->1, dd8cigql->11")
    print("=" * 64)

    # members.user_id
    cur.execute("SELECT user_id, user_name FROM members")
    for user_id, user_name in cur.fetchall():
        if user_id in USER_MAP:
            new_id = USER_MAP[user_id]
            cur.execute("UPDATE members SET user_id=%s WHERE user_id=%s", (new_id, user_id))
            total += 1
            print(f"[members.user_id] {user_name}: {user_id} -> {new_id}")

    # JSON 字段里的 userid 替换：用 SQL REPLACE 精确替换 JSON 字符串值
    json_fields = [
        ("duty_schedules", "users"),
        ("duty_managements", "manager"),
        ("duty_managements", "cur_duty_user"),
        ("duty_calendar_infos", "user_group"),
    ]
    for t, col in json_fields:
        for old_uid, new_uid in USER_MAP.items():
            try:
                cur.execute(
                    f"UPDATE `{t}` SET {col} = REPLACE({col}, %s, %s) WHERE {col} LIKE %s",
                    (f'"{old_uid}"', f'"{new_uid}"', f'%"{old_uid}"%'),
                )
                if cur.rowcount > 0:
                    total += cur.rowcount
                    print(f"[{t}.{col}] userid {old_uid}->{new_uid}: {cur.rowcount} 行")
            except Exception as e:
                print(f"[{t}.{col}] userid {old_uid}->{new_uid} 跳过: {e}")

    conn.commit()
    print("=" * 64)
    print(f"迁移完成，总受影响行数：{total}")

    # 验证
    print("\n===== 验证 =====")
    cur.execute("SELECT user_id, user_name, role, tenants FROM members")
    print("members:", cur.fetchall())
    for t in ["duty_schedules", "duty_managements", "duty_calendar_infos"]:
        cur.execute(f"SELECT tenant_id FROM `{t}` GROUP BY tenant_id")
        print(f"{t} tenant_id 分布:", cur.fetchall())
    cur.execute("SELECT users FROM duty_schedules WHERE users IS NOT NULL LIMIT 1")
    print("duty_schedules.users 示例:", cur.fetchall())
    cur.execute("SELECT manager FROM duty_managements WHERE manager IS NOT NULL LIMIT 1")
    print("duty_managements.manager 示例:", cur.fetchall())

    conn.close()


if __name__ == "__main__":
    main()
