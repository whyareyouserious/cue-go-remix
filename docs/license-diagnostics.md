# Cue 授权连接与日志排查

2026-11-12 对照 `/Users/root1/JavaDemo/service-duan` 的控制器、鉴权配置与已上线接口确认：

- 默认 API 根地址：`http://43.142.63.76/api`。
- 旧产品使用 `/api/license/*` 和 `/api/trial/*`；Cue 使用 `/api/cue/license/*` 和 `/api/cue/trial/*`，全部是 POST。
- Cue 激活、心跳、注销分别为 `license/activate`、`license/heartbeat`、`license/deactivate`；试用为 `trial/activate`、`trial/heartbeat`。
- 公网 80 端口可转发上述接口；当前 8088 Nginx 白名单没有 Cue 前缀，返回 404。不要把客户端改成旧产品接口，旧接口会拒绝 Cue 卡和 Token。
- systemd 运行 `/opt/license-server/license-server.jar`，通过 `SERVER_ADDRESS=127.0.0.1` 监听本机 8088，Nginx 转发到该地址。

旧构建的 `LicenseBaseURL` 为空且正式版拒绝 HTTP，请求尚未发出便提示“授权服务地址未配置或不支持”。新版默认使用上述固定部署入口，其他自定义发行地址要求 HTTPS；启动时环境变量不会覆盖正式版地址。HTTP 兼容入口没有传输加密，后续部署 HTTPS 后重新构建即可切换。

## 客户端日志

macOS 默认日志：`~/Library/Application Support/cue-go-remix/logs/cue.log`。
Windows 默认日志：`%APPDATA%\cue-go-remix\logs\cue.log`。
设置了 `CUE_DATA_DIR` 时，以该目录下的 `logs/cue.log` 为准。原生验收时的临时数据目录不等于用户正常启动的目录。

macOS 查看最近授权事件：

```bash
grep -E 'event=license-(config|request|validation|state)' \
  "$HOME/Library/Application Support/cue-go-remix/logs/cue.log" | tail -n 80
```

Windows PowerShell：

```powershell
Get-Content "$env:APPDATA\cue-go-remix\logs\cue.log" -Tail 200 |
  Select-String 'event=license-'
```

每行有时间与进程 PID。`license-config` 记录授权服务的协议/主机、配置是否有效及 HTTP 是否允许；`license-request` 记录固定操作名、HTTP 状态、业务码、结果分类和耗时。`http_status=0` 表示没有收到 HTTP 响应，`api_code=-1` 表示尚未取得业务码。日志不包含卡密、Token、机器标识、任意 URL 路径/查询参数、请求响应正文或底层异常原文。

| 日志 | 含义与下一步 |
| --- | --- |
| `valid=false` / `result=invalid_endpoint` | 地址配置无效，请确认运行的是新版构建 |
| `result=timeout/dns/connection_refused/network` | 请求未完成，按分类检查客户端网络与服务监听 |
| `result=tls_certificate` | HTTPS 证书验证失败，检查域名与证书 |
| `http_status=404` | 入口路径未转发或后端版本不对，检查是否错误使用 8088 |
| `http_status=502/503` | Nginx 上游不可用，检查 Java 服务状态及启动日志 |
| `api_code=1301` | 请求参数不符合协议；空 JSON 连通性探针预期得到此码 |
| `api_code=1302` | 请求到达服务端但内部处理失败；按时间查 Java 日志及增量迁移 |
| `api_code=1001/1006/1007` | 卡不存在 / 非 Cue 卡 / 已绑定其他电脑 |
| `result=invalid_response` | 接口返回非预期 JSON 或没有业务码，检查路由及响应类型 |
| `event=license-validation ... result=invalid_payload` | 服务返回 code=0，但产品、模式、有效期或 Token 等协议字段不符合 Cue 约定 |
| `result=ok` | HTTP 和业务封装成功；最终授权以 `license-state ... allowed=true` 为准 |

## 服务端日志与连通性检查

```bash
systemctl status license-server --no-pager -l
journalctl -u license-server --since '10 minutes ago' --no-pager
```

Java 现有 Log4j2 默认同时写控制台和工作目录的 `log/current.log`；在当前 systemd 工作目录下即 `/opt/license-server/log/current.log`，除非部署覆盖了 `APP_LOG_DIR` / `APP_LOG_FILE`。Cue 的 `ApiLogAspect` 记录接口路径、结果、错误码和耗时。Nginx 拒绝的请求不会进入 Java，需要查看对应站点的 access/error log。

只检查连通性，不输入卡密，也不领取试用：

```bash
curl --connect-timeout 4 --max-time 8 -i \
  -H 'Content-Type: application/json' -d '{}' \
  http://43.142.63.76/api/cue/license/activate
```

预期 HTTP 200 与业务码 1301。此结果只证明接口路由与参数校验可达，不证明数据库迁移、真实激活、绑机或试用成功。若实际激活返回 1302，再根据服务端日志核对 `bound_machine_id` 和 `cue_trial_record` 的增量迁移，不能直接覆盖生产数据库。
