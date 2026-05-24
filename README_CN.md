# amdtop

AMD GPU 终端监控工具，支持 Linux 和 Windows。

[English Documentation](README.md)

![Example](Example.jpg)

## 功能

- 实时显示 GPU 核心利用率、温度、功耗、显存占用
- 弹簧动画平滑进度条与柱状图
- 历史图表，可在利用率/温度/功耗/显存间切换
- 终端高度不足时自动启用紧凑模式（进度条与图表二选一）
- Server / Client 模式，通过 HTTP API 远程监控
- 所有颜色可通过配置文件自定义

## 下载

预编译二进制文件可从 [Releases 页面](https://github.com/zyoung11/amdtop/releases) 下载。

### Linux

下载 `amdtop-linux-amd64`，添加执行权限后直接运行：

```bash
chmod +x amdtop-linux-amd64
./amdtop-linux-amd64
```

无需额外依赖，直接从 sysfs 读取 amdgpu 驱动数据。

### Windows

下载 `amdtop-windows-amd64.exe` 直接运行。需要 AMD 显卡驱动（提供 `ADLX.dll`，位于 `C:\Windows\System32\`）。

### 从源码编译

```bash
git clone https://github.com/zyoung11/amdtop.git
cd amdtop
```

**Linux：**

```bash
go build -ldflags="-s -w" .
```

**Windows**（需要 [MinGW-w64](https://www.mingw-w64.org/) 编译 cgo）：

```powershell
go build -ldflags="-s -w" .
```

## 使用

```
amdtop                        本地 TUI 模式
amdtop -s -p <端口>            服务端模式（TUI + HTTP API）
amdtop -s -n -p <端口>         无头服务端模式（无 TUI，适用于服务管理）
amdtop -c -i <IP> -p <端口>    客户端模式（连接远程服务端）
amdtop -h, --help             显示帮助
```

### 本地模式

```bash
./amdtop
```

操作：`Tab` 切换图表数据，`q` 退出。

### 服务端模式

启动 TUI 并在指定端口提供 HTTP API。

```bash
./amdtop -s -p 16969
```

端口会在首次使用后保存到配置文件，之后可省略 `-p`。

API 地址：`GET /api/v1/metrics` 返回包含所有 GPU 指标的 JSON。

### 无头服务端模式

仅启动 HTTP API，不启动 TUI 界面，适合注册为系统服务使用。

```bash
./amdtop -s -n -p 16969
```

服务在后台轮询 GPU 数据并通过 HTTP API 提供。
可用于 `nssm`（Windows）或 `systemd`（Linux）等服务管理器。

### 客户端模式

连接远程服务端，显示其 GPU 数据。

```bash
./amdtop -c -i 192.168.100.1 -p 16969
```

IP 和端口会在首次使用后保存，之后可省略 `-i` 和/或 `-p`。

## 配置

配置文件位于 `~/.config/amdtop/config.json`，首次运行自动创建。

```json
{
  "title_color": "#e65100",
  "gauges": {
    "gpu":   "default",
    "temp":  "default",
    "power": "default",
    "vram":  "default"
  },
  "charts": {
    "util":  "#e65100",
    "temp":  "#e65100",
    "power": "#e65100",
    "vram":  "#e65100"
  },
  "default_chart": "util",
  "server_color": "#4aa84a",
  "client_color": "#58a6ff",
  "poll_interval_ms": 1000,
  "client_poll_interval_ms": 1000
}
```

### 进度条颜色

填写 `"default"` 则根据数值自动切换绿/黄/红。
填写具体色值（如 `"#e65100"`）则固定为该颜色。

### 图表颜色

四种图表（util、temp、power、vram）各有独立颜色，默认均为 `#e65100`（AMD 橙）。

### 功耗上限

- `power_cap_w` — 手动设置功耗上限（瓦特），默认 `0` 表示自动从驱动检测。
  设置为大于 0 的值时，功率进度条的百分比和图表缩放将使用此值替换驱动检测到的上限。
  适用于驱动不报告功耗上限的情况（如部分 APU）。

### 轮询间隔

- `poll_interval_ms` — 本地/服务端模式的数据采集间隔（默认 `1000` 毫秒），最小 `100`。
- `client_poll_interval_ms` — 客户端模式的 HTTP 轮询间隔（默认 `1000` 毫秒），最小 `100`。

### 服务端 / 客户端连接

```json
"server": {
  "port": 16969
},
"client": {
  "ip": "192.168.100.1",
  "port": 16969
}
```

- `server.port` — 服务端监听端口。使用 `-s -p <端口>` 时自动设置，也可手动编辑。
- `client.ip` — 远程服务端 IP 地址。使用 `-c -i <IP>` 时自动设置，也可手动编辑。
- `client.port` — 远程服务端端口。使用 `-c -p <端口>` 时自动设置，也可手动编辑。

## 协议

MIT
