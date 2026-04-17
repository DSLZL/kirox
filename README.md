# Kiro Registration Client

<p align="center">
  <img src="build/appicon.svg" width="128" height="128" alt="Kiro_reg Logo">
</p>

<p align="center">
  <strong>AWS Builder(Kiro) ID 自动注册工具</strong>
</p>

<p align="center">
  <a href="https://github.com/huey1in/kiro_reg/actions"><img src="https://github.com/huey1in/kiro_reg/actions/workflows/build.yml/badge.svg" alt="Build Status"></a>
  <a href="https://github.com/huey1in/kiro_reg/releases"><img src="https://img.shields.io/github/v/release/huey1in/kiro_reg" alt="Release"></a>
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-blue" alt="Platform">
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go" alt="Go Version">
</p>

---

### 从源码构建

#### 前置要求

- [Go 1.24+](https://go.dev/dl/)
- [Node.js 20+](https://nodejs.org/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 克隆仓库
git clone https://github.com/huey1in/kiro_reg.git
cd kiro_reg

# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式
wails dev

# 生产构建
wails build
```

## 🏗️ 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.24 |
| 前端 | Vue.js |
| 桌面框架 | [Wails v2](https://wails.io/) |
| HTTP 客户端 | TLS-Client (反指纹) |
| 加密 | CBOR + AES |

## 项目结构

```
├── app.go              # 主应用逻辑
├── main.go             # 入口点
├── main_windows.go     # Windows 平台特定代码
├── wails.json          # Wails 配置
├── internal/           # 内部包
│   ├── api/            # API 处理
│   ├── license/        # 许可证管理
│   ├── security/       # 安全与加密
│   └── storage/        # 数据存储
├── frontend/           # Vue.js 前端
│   ├── src/
│   └── assets/
└── build/              # 构建资源
    └── appicon.svg     # 应用图标
```

## License

Copyright © 2026. All rights reserved.
