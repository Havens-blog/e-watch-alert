# -*- coding: utf-8 -*-
"""WatchAlert 租户 ID 迁移脚本：本地 "default" -> eiam default-tenant (id=2)。

迁移内容：
1. tenants 表：id "default" -> "2"
2. tenant_linked_users 表：id "default" -> "2"
3. members 表：tenants JSON ["default","default"] -> ["2"]（去重）
4. 所有业务表 tenant_id "default" -> "2"

设计为幂等：重复执行不会产生副作用（已迁移的数据不会再次迁移）。
执行前会打印受影响行数，执行后打印验证结果。
"""
import json
import pymysql

DB = dict(host="106.52.187.69", port=3306, user="root", password="123456",
          database="watchalert", charset="utf8mb4")

OLD = "default"
NEW = "2"  # eiam default-tenant id

# tenant_id 字段的表（来自 SHOW COLUMNS 探查）
TENANT_ID_TABLES = [
    "alert_cur_events", "alert_data_sources", "alert_his_events", "alert_notices",
    "alert_rules", "alert_silences", "audit_logs", "comments", "dashboard_folders",
    "dashboards", "duty_calendar_infos", "duty_managements", "duty_schedules",
    "notice_records", "rule_groups", "w8t_fault_center", "w8t_probe_rule",
    "w8t_prometheus_target", "w8t_prometheus_target_group", "w8t_prometheus_target_version",
    "w8t_recording_rule_groups", "w8t_recording_rules",
]


def main():
    conn = pymysql.connect(**DB)
    cur = conn.cursor()
    total_affected = 0

    print("=" * 60)
    print("开始迁移：tenant_id/租户 id  'default' -> '2' (eiam default-tenant)")
    print("=" * 60)

    # 1. tenants 表
    cur.execute("SELECT COUNT(*) FROM tenants WHERE id=%s", (OLD,))
    cnt = cur.fetchone()[0]
    if cnt > 0:
        cur.execute("UPDATE tenants SET id=%s WHERE id=%s", (NEW, OLD))
        total_affected += cnt
        print(f"[1] tenants 表：更新 {cnt} 行 (id default->2)")
    else:
        print("[1] tenants 表：无需迁移（可能已迁移）")

    # 2. tenant_linked_users 表
    cur.execute("SELECT COUNT(*) FROM tenant_linked_users WHERE id=%s", (OLD,))
    cnt = cur.fetchone()[0]
    if cnt > 0:
        cur.execute("UPDATE tenant_linked_users SET id=%s WHERE id=%s", (NEW, OLD))
        total_affected += cnt
        print(f"[2] tenant_linked_users 表：更新 {cnt} 行")
    else:
        print("[2] tenant_linked_users 表：无需迁移")

    # 3. members 表 tenants JSON 字段
    cur.execute("SELECT user_id, tenants FROM members WHERE tenants IS NOT NULL")
    for user_id, tenants_json in cur.fetchall():
        try:
            arr = json.loads(tenants_json)
        except (TypeError, json.JSONDecodeError):
            continue
        if not isinstance(arr, list):
            continue
        new_arr = [NEW if (x == OLD or x is None or x == "") else x for x in arr]
        # 去重并保序
        seen = set()
        dedup = []
        for x in new_arr:
            if x not in seen:
                seen.add(x)
                dedup.append(x)
        if dedup != arr:
            cur.execute("UPDATE members SET tenants=%s WHERE user_id=%s",
                        (json.dumps(dedup, ensure_ascii=False), user_id))
            total_affected += 1
            print(f"[3] members 表：user_id={user_id} tenants {arr} -> {dedup}")
    print("[3] members 表：迁移完成")

    # 4. 业务表 tenant_id
    for t in TENANT_ID_TABLES:
        try:
            cur.execute(f"SELECT COUNT(*) FROM `{t}` WHERE tenant_id=%s", (OLD,))
            cnt = cur.fetchone()[0]
            if cnt > 0:
                cur.execute(f"UPDATE `{t}` SET tenant_id=%s WHERE tenant_id=%s", (NEW, OLD))
                total_affected += cnt
                print(f"[4] {t}：更新 {cnt} 行 tenant_id default->2")
        except Exception as e:
            print(f"[4] {t}：跳过（{e}）")

    # 5. alert_subscribes 表 s_tenant_id
    try:
        cur.execute("SELECT COUNT(*) FROM alert_subscribes WHERE s_tenant_id=%s", (OLD,))
        cnt = cur.fetchone()[0]
        if cnt > 0:
            cur.execute("UPDATE alert_subscribes SET s_tenant_id=%s WHERE s_tenant_id=%s", (NEW, OLD))
            total_affected += cnt
            print(f"[5] alert_subscribes：更新 {cnt} 行 s_tenant_id")
    except Exception as e:
        print(f"[5] alert_subscribes：跳过（{e}）")

    conn.commit()
    print("=" * 60)
    print(f"迁移完成，总受影响行数：{total_affected}")

    # 验证
    print("\n===== 验证 =====")
    cur.execute("SELECT id, name FROM tenants")
    print("tenants:", cur.fetchall())
    cur.execute("SELECT id FROM tenant_linked_users")
    print("tenant_linked_users ids:", cur.fetchall())
    cur.execute("SELECT user_id, tenants FROM members")
    print("members tenants:", cur.fetchall())
    # 抽查几个业务表
    for t in ["alert_rules", "duty_schedules", "notice_records", "audit_logs"]:
        cur.execute(f"SELECT tenant_id, COUNT(*) FROM `{t}` GROUP BY tenant_id")
        print(f"{t} tenant_id 分布:", cur.fetchall())

    conn.close()


if __name__ == "__main__":
    main()
