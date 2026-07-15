# 恢复管理员访问权限

仅当一个**已启用的管理员**丢失全部登录因子，并且没有其他管理员可以使用“重置访问权限”时，才使用 BreakGlass。它不能修复配置、数据库或启动失败。

BreakGlass 会替换管理员密码，删除通行密钥、TOTP 和恢复码，并使已有会话失效。使用临时密码登录后，必须立即设置永久密码。

::: danger 敏感日志
临时密码只写入 `security.log`。不要上传该文件、粘贴到 Issue，或将它发送到日志收集服务。
:::

## Docker Compose

在包含 Lumilio Photos Compose 文件的目录中执行以下命令。

1. 停止正常 Server，避免同时运行两个队列和 API 实例：

   ```bash
   docker compose stop server
   ```

2. 启动一次性恢复容器。不指定用户名时，将恢复最早创建的已启用管理员：

   ```bash
   docker compose run -d --name lumilio-breakglass \
     -e LUMILIO_BREAK_GLASS=true \
     -e LUMILIO_BREAK_GLASS_USERNAME=admin \
     server
   ```

3. 读取成功的 `auth.break_glass` 事件，并复制其中的 `temporary_password`：

   ```bash
   docker exec lumilio-breakglass cat /app/logs/security.log
   ```

4. 删除一次性容器，并在不启用 BreakGlass 的情况下重新启动正常 Server：

   ```bash
   docker rm -f lumilio-breakglass
   docker compose up -d server
   ```

5. 使用临时密码登录，并在提示时设置永久密码。

## Desktop

首先从菜单栏或系统托盘中完全退出 Lumilio Photos。已有实例仍在运行时，恢复启动会被拒绝。

### macOS

```bash
open -n -a "Lumilio Photos" --args \
  --break-glass \
  --break-glass-username admin
```

安全日志位置：

```text
~/Library/Application Support/Lumilio Photos/logs/security.log
```

### Windows PowerShell

```powershell
& "$env:LOCALAPPDATA\Programs\Lumilio Photos\lumilio-photos.exe" `
  --break-glass `
  --break-glass-username admin
```

安全日志位置：

```text
%LOCALAPPDATA%\Lumilio Photos\logs\security.log
```

删除 `--break-glass-username admin` 即可恢复最早创建的已启用管理员。复制临时密码后，退出本次恢复启动，再正常打开 Lumilio Photos。随后使用临时密码登录并完成强制改密。

## 恢复失败时

- 指定的账户必须存在、具有管理员角色并处于启用状态。
- Desktop 用户应确认原有托盘应用已经完全退出。
- Docker 用户可通过 `docker logs lumilio-breakglass` 检查启动错误，并等待 `security.log` 创建完成。
- 如果配置加载、PostgreSQL、迁移或安全日志初始化失败，应先修复该启动问题；BreakGlass 只会在这些依赖就绪后运行。
