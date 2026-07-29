---
title: HTTPS 与通行密钥
description: 在基础部署完成后，为 Lumilio 添加 HTTPS、域名或现有反向代理。
---

# HTTPS 与通行密钥

Lumilio 不要求你在安装前购买域名。Linux Server 可以先通过 `http://主机IP:6680` 完成部署；以后使用内网域名、DDNS、IPv4 直连或内网穿透时，也不需要先把“正式 URL”写入 Lumilio。

Lumilio 会根据浏览器当前访问的地址识别协议和主机名：

- `http://localhost:6680` 可以使用通行密钥；
- `http://192.168.1.20:6680` 可以使用密码和 TOTP，但不能使用通行密钥；
- `https://photos.example.com` 可以使用通行密钥；
- `https://203.0.113.10` 即使证书有效，也不能作为通行密钥的 RP ID，需要改用域名。

通行密钥会绑定登记时使用的域名。以后换成另一个域名时，原有通行密钥不会自动迁移；先用密码与 TOTP 登录，再在新域名下登记新的通行密钥。

## 已有反向代理

如果 NAS 或服务器已经运行 Caddy、Nginx、Traefik、Nginx Proxy Manager 等工具，可以继续使用默认的 `compose.yml`。把代理上游设置为：

```text
http://127.0.0.1:6680
```

代理和 Lumilio 不在同一台主机时，把 `127.0.0.1` 换成 Lumilio 主机的局域网地址，并使用防火墙限制谁能访问 6680。

反向代理需要覆盖客户端传入的转发头，并传递实际的协议与主机名。使用以下任意一组标准信息即可：

- `Forwarded: proto=https;host=photos.example.com`
- `X-Forwarded-Proto: https` 与 `X-Forwarded-Host: photos.example.com`

`X-Forwarded-For` 只影响日志和限流所看到的客户端 IP，不决定请求能否访问 Lumilio。只有需要恢复真实客户端 IP 时，才在自定义 Server manifest 的 `server.proxy.trusted_cidrs` 中填写代理地址。

仓库的 `deploy/reverse-proxy/` 目录提供 Caddy、Nginx 和 Traefik 示例。替换示例域名与证书路径后再使用。

## 使用随附的 Caddy

如果还没有反向代理，并且域名已经指向这台 Linux 主机，可以使用随附的 Caddy Compose：

```bash
docker compose down
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.caddy.yml
export LUMILIO_DOMAIN=photos.example.com
docker compose -f compose.caddy.yml up -d
```

Caddy 监听主机的 80/443，自动申请证书，并把请求转发给只监听 `127.0.0.1:6680` 的 Lumilio。媒体和应用状态默认仍位于 `./lumilio/media` 与 `./lumilio/app-state`；也可以继续使用 `LUMILIO_STORAGE` 和 `LUMILIO_STATE` 指定原来的目录。

## 使用内置 ACME

内置 ACME 适合让 Lumilio 直接监听 80/443。开始前确认：

- 域名的 A/AAAA 记录已经指向这台主机；
- 公网能够访问 TCP 80 和 443；
- 这两个端口没有被其他服务占用。

先生成一份 ACME manifest，再启动对应的 Compose：

```bash
docker compose down
export LUMILIO_IMAGE=ghcr.io/edwinzhancn/lumilio-server:latest
export LUMILIO_STATE="$(pwd)/lumilio/app-state"
mkdir -p "$LUMILIO_STATE"

docker run --rm \
  -v "$LUMILIO_STATE:/data/app-state" \
  "$LUMILIO_IMAGE" \
  server config init \
  --profile docker-acme \
  --hostname photos.example.com \
  --email admin@example.com \
  --output /data/app-state/server.toml

curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.acme.yml
docker compose -f compose.acme.yml up -d
```

替换示例中的域名和邮箱。证书申请失败时，Server 会停止启动，不会自动降级到 HTTP。

## Desktop 从其他设备访问

Desktop 本机默认使用 `http://localhost:6680`，不需要证书。若要从其他设备访问：

1. 在 Desktop 控制面板中把访问方式改为“局域网 HTTP”，阅读并接受提示；
2. 先确认另一台设备能够打开 `http://Desktop主机IP:6680`；
3. 如需加密，再让现有反向代理转发到这个局域网地址；
4. 使用代理提供的 HTTPS 域名打开 Lumilio。

Desktop 控制面板不管理域名、证书或反向代理。远程代理场景应使用防火墙只允许代理访问 Desktop 的 6680 端口。

::: warning 内网穿透与端口转发
不要把 6680 的明文 HTTP 端口直接暴露到公网。IPv4 直连、路由器端口转发和内网穿透服务应在外层提供 HTTPS；如果服务只给出临时或经常变化的域名，每次换域名后都需要重新登记通行密钥。
:::
