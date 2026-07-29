---
title: 恢复管理员访问
---

# 恢复管理员访问

只有当一个**仍处于启用状态的管理员**丢失全部登录因素，且没有其他管理员可以重置其访问权限时，才使用 BreakGlass。它会重置管理员密码，移除通行密钥、TOTP 与恢复代码，并使现有会话失效。

开始前先确认正在操作正确的实例，并创建一份最新的 SQLite 数据库快照。BreakGlass 不能修复配置、数据库或启动故障。

::: danger 临时密码属于敏感信息
临时密码只写入 `security.log`。不要上传这个文件、粘贴到 Issue，或发送给日志收集服务。
:::

## Docker Compose

以下命令必须在部署目录执行，并使用生产环境实际选择的同一个 Compose 文件。

```bash
export COMPOSE_FILE=/path/to/compose.yml
docker compose stop
docker compose run --no-deps -d --name lumilio-breakglass \
  -e LUMILIO_BREAK_GLASS=true \
  -e LUMILIO_BREAK_GLASS_USERNAME=admin \
  lumilio
docker exec lumilio-breakglass cat /data/app-state/logs/security.log
```

复制成功的 `auth.break_glass` 事件中的 `temporary_password` 后：

```bash
docker rm -f lumilio-breakglass
docker compose up -d
```

如果实际使用 `compose.caddy.yml` 或 `compose.acme.yml`，相应替换 `COMPOSE_FILE`。省略 `LUMILIO_BREAK_GLASS_USERNAME` 会恢复最早创建的启用中管理员。

## Desktop：macOS

先从菜单栏完全退出 Lumilio Photos，再运行：

```bash
open -n -a "Lumilio Photos" --args \
  --break-glass \
  --break-glass-username admin
```

临时密码位于：

```text
~/Library/Application Support/Lumilio Photos/logs/security.log
```

## Desktop：Windows PowerShell

先从托盘完全退出 Lumilio Photos，再运行：

```powershell
& "$env:LOCALAPPDATA\Programs\Lumilio Photos\lumilio-photos.exe" `
  --break-glass `
  --break-glass-username admin
```

临时密码位于：

```text
$env:LOCALAPPDATA\Lumilio Photos\logs\security.log
```

省略 `--break-glass-username admin` 会恢复最早创建的启用中管理员。

## 完成恢复

退出这次恢复启动，正常启动 Lumilio Photos，用临时密码登录并立即设置永久密码，然后重新登记 TOTP 或通行密钥并生成新恢复代码。

如果恢复失败，确认目标账户存在、角色为管理员且仍启用；Desktop 必须已完全退出。Docker 可运行 `docker logs lumilio-breakglass` 检查启动失败。配置、SQLite 迁移或安全日志初始化失败时，必须先修复该启动问题。
