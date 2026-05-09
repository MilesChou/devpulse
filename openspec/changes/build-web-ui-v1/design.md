## Context

待 `build-web-ui-poc` 完成、收集使用情境與限制後再填入。

## Goals / Non-Goals

**Goals：** 待定

**Non-Goals：** 待定

## Decisions

待定。預計需要拍板的方向：

- 自訂報表的篩選模型與 UI 抽象
- 匯出（CSV / PDF）採用的工具與容器影響
- URL 狀態序列化策略（壓縮、長度限制）
- 排程寄送的時區、SMTP 來源、retry 策略
- OAuth 白名單預設行為、session 儲存
- read-only DB role 與 schema 邊界

## Risks / Trade-offs

待定。

## Open Questions

- v1 是否一次到位涵蓋全部候選功能？或分多次 change？
- 是否需要從 PoC 共用 write user 切換為 read-only？切換時機？
