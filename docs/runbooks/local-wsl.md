# 本机 Linux 打包运行（WSL）

在本机 WSL 的 Ubuntu 里按服务器的方式跑一遍：Nginx 直出静态站并把 `/api/v1/` 转给 Go，Go 只监听 `127.0.0.1:8080`。不部署到任何服务器，产物全在 `/srv/blog`，不碰仓库。

## 一次性准备

1. WSL 用 root 直接跑（不建普通用户）：安装 Ubuntu 时在 "Enter new UNIX username" 处按 Ctrl+C 退出即可，发行版已装好且只有 root。再把默认用户固定为 root、打开 systemd：

   ```powershell
   wsl -d Ubuntu -u root -- sh -c 'printf "[user]\ndefault=root\n\n[boot]\nsystemd=true\n" > /etc/wsl.conf'
   ```

2. Windows 侧 `%USERPROFILE%\.wslconfig`（已写）：`networkingMode=mirrored` + `autoProxy=true`。作用：本机的 localhost 代理（如 127.0.0.1:7890）在 WSL 里可用，去掉 "localhost 代理配置未镜像到 WSL" 的警告；WSL 里的 8088/8080 端口在 Windows 上就是 localhost。改完要 `wsl --shutdown` 一次。

3. Windows 上构建产物（在仓库根，Git Bash；PowerShell 用 `$env:` 形式）：

   ```bash
   MSYS_NO_PATHCONV=1 PUBLIC_SITE_URL=http://localhost:8088 PUBLIC_ASSET_BASE=/assets PUBLIC_API_BASE= pnpm -C site build
   cd api && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/blog-server-linux ./cmd/server
   ```

   `PUBLIC_ASSET_BASE=/assets` 与空的 `PUBLIC_API_BASE` 就是生产的同源配置：资产与 API 都走 Nginx。Git Bash 必须加 `MSYS_NO_PATHCONV=1`，否则 `/assets` 会被转成 Windows 路径。

## 启动与停止

```powershell
wsl -d Ubuntu -- sh /mnt/d/Work/blog/newsite/deploy/scripts/local-wsl.sh
wsl -d Ubuntu -- sh /mnt/d/Work/blog/newsite/deploy/scripts/local-wsl-stop.sh
```

脚本做的事：apt 装 nginx（首次）；把 `site/dist`、Linux 二进制、`api/data`（本地资产与数据库）复制到 `/srv/blog`；用 `api/.env` 生成 `/srv/blog/.env`（改 `DATA_DIR`、`CORS_ORIGINS` 等）；把 `deploy/nginx/blog.conf` 填上端口与目录装进 `sites-enabled`；后台启动 Go；打印健康检查。重新构建后再跑一次脚本即发布新产物。

## 测试入口（Windows 浏览器）

| 地址 | 说明 |
|---|---|
| http://localhost:8088/ | 站点首页（Nginx） |
| http://localhost:8088/posts/hello-world/ | Hello World 测试文章，列出了所有页面与机器入口 |
| http://localhost:8088/api/v1/counters?slugs=hello-world | 经 Nginx 转发到 Go 的公开接口 |
| http://localhost:8088/index.php/blog/ | 旧站路径 301 到 `/posts/`；`/index.php/download/` 返回 410 |
| http://localhost:8088/admin/ | 404：后台不经 Nginx 暴露 |
| http://localhost:8080/admin/ | 后台直连（模拟 SSH 隧道），用户 `admin`，密码见 `api/.env` 生成时用的明文 |
| http://localhost:8080/healthz | Go 健康检查 |

日志：`/srv/blog/api.log`（Go）、`/var/log/nginx/error.log`。Go 数据在 `/srv/blog/data`，与 Windows 的 `api/data` 是两份拷贝，互不影响。

## 与真实服务器的差别

- 真实部署走 `deploy/systemd/blog-api.service`（`User=blog`，`EnvironmentFile=/srv/blog/.env`）而不是 `nohup`；Nginx 监听 443 并把 `/assets/` 302 到 CDN 而不是代理给 Go；构建在服务器上由 `RELEASE_SCRIPT` 执行，这里的 `release.sh` 只是占位。
- Nginx 里 `add_header` 在 `location` 内会覆盖 server 级的安全头，阶段 7 整理时统一处理。
