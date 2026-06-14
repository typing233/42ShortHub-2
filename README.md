# ShortHub - 短链接服务

生产化短链接服务，支持 REST API 和 Web 管理界面。

## 技术栈

- **后端**: Go 1.22 + Gin
- **数据库**: PostgreSQL 16
- **缓存**: Redis 7
- **容器化**: Docker Compose

## 功能特性

- 短链接的创建、查看、修改、删除、搜索
- 自定义短码 / 随机生成（Base62，高并发唯一性保证）
- 过期时间设置、启停状态控制
- 批量创建短链接
- 访问跳转 + 异步日志记录（IP / UA / Referer / 时间）
- JWT 登录鉴权 + 多用户权限隔离
- Redis 缓存热点短码 + 接口限流
- URL 合法性校验 + 防滥用（禁止内网地址）
- Web 管理面板

## 快速开始

### Docker Compose 一键启动

```bash
docker compose up --build -d
```

服务启动后访问 http://localhost:8080/register 注册账号，然后登录进入管理面板。

### 本地开发

需要先启动 PostgreSQL 和 Redis：

```bash
# 仅启动依赖
docker compose up postgres redis -d

# 运行服务
export DB_HOST=localhost REDIS_ADDR=localhost:6379
go run ./cmd/server
```

### 运行测试

项目有三个测试层级：

```bash
# 1. 单元测试 —— 无外部依赖，随时可跑
make test-unit

# 2. 集成测试 —— 需要 PostgreSQL + Redis
#    先启动依赖：
make docker-deps
#    然后运行：
make test-integration

# 3. Docker 全链路 E2E 冒烟测试 —— 自动 build 镜像并验证 10 个检查点
make test-e2e

# 全部（单元 + 集成，需依赖运行中）：
make test
```

| 层级 | 命令 | 外部依赖 | 位置 |
|------|------|----------|------|
| 单元测试 | `make test-unit` | 无 | `internal/*/...` |
| 集成测试 | `make test-integration` | PG + Redis | `tests/integration_test.go` |
| E2E 冒烟 | `make test-e2e` | Docker Compose | `tests/e2e_docker_test.sh` |

> 没有外部依赖时，集成测试会自动 `t.Skip`，不会报错。

## API 文档

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/auth/register | 注册 |
| POST | /api/v1/auth/login | 登录，返回 JWT |
| POST | /api/v1/auth/logout | 退出 |

### 短链接管理（需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/links | 创建短链接 |
| POST | /api/v1/links/batch | 批量创建 |
| GET | /api/v1/links | 列表查询（支持分页、搜索、状态过滤） |
| GET | /api/v1/links/:id | 获取详情 |
| PUT | /api/v1/links/:id | 修改（标题/状态/过期时间） |
| DELETE | /api/v1/links/:id | 删除 |

### 跳转

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /s/:code | 301 跳转到原始 URL |

## 请求示例

```bash
# 注册
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","email":"demo@example.com","password":"123456"}'

# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"123456"}'

# 创建短链接
curl -X POST http://localhost:8080/api/v1/links \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://github.com","custom_code":"gh","title":"GitHub"}'

# 批量创建
curl -X POST http://localhost:8080/api/v1/links/batch \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"links":[{"url":"https://google.com"},{"url":"https://github.com"}]}'

# 访问短链接
curl -L http://localhost:8080/s/gh
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| SERVER_PORT | 8080 | 服务端口 |
| DB_HOST | localhost | PostgreSQL 主机 |
| DB_PORT | 5432 | PostgreSQL 端口 |
| DB_USER | shortlink | 数据库用户 |
| DB_PASSWORD | shortlink | 数据库密码 |
| DB_NAME | shortlink | 数据库名 |
| REDIS_ADDR | localhost:6379 | Redis 地址 |
| JWT_SECRET | change-me | JWT 签名密钥 |
| APP_BASE_URL | http://localhost:8080 | 服务基础 URL |
| SHORT_CODE_LEN | 6 | 随机短码长度 |
| RATE_LIMIT_PER_MIN | 60 | 每 IP 每分钟请求限制 |
| MAX_BATCH_SIZE | 50 | 批量创建最大数量 |

## 架构说明

```
cmd/server/main.go          -- 入口，路由注册
internal/
  config/                   -- 配置加载
  model/                    -- 数据模型 + DTO
  repository/               -- 数据访问层
  service/                  -- 业务逻辑层
  handler/                  -- HTTP 处理器
  middleware/               -- JWT 鉴权 + 限流
  cache/                    -- Redis 封装
web/
  templates/                -- HTML 模板
  static/                   -- CSS + JS
```

### 高并发短码唯一性

1. 随机生成 + 数据库 UNIQUE 约束（最终防线）
2. 生成前先查询是否存在，最多重试 10 次
3. 自定义短码冲突时返回 409 + 明确错误提示
4. 通过 DB unique index 在并发下保证绝对唯一

### 异步日志写入

跳转时将访问记录投入 buffered channel（容量 10000），由 4 个 goroutine 消费者批量写入数据库（每 100 条或每 2 秒刷盘），不阻塞 301 响应。

**可靠性保证：**

| 场景 | 处理方式 |
|------|----------|
| 数据库临时不可用 | 指数退避重试 3 次（500ms → 1s → 2s） |
| 重试全部失败 | 写入本地 fallback 文件 (`access_log_fallback.tsv`) + stderr 输出，不会无声丢失 |
| 服务正常退出 (SIGTERM/SIGINT) | 先 drain channel 中所有缓冲记录再退出 |
| Channel 满（极端高并发） | 丢弃新条目，原子计数器计数，每 1000 条打印一次警告 |

**仍可能丢失的情况：**

1. **SIGKILL / OOM Kill**：进程被强制杀死，channel 中未消费的条目丢失（无法避免）
2. **Shutdown 超时**：如果 5 秒内无法刷完积压，剩余条目丢失
3. **Fallback 文件磁盘满**：落盘文件写入失败时 stderr 仍会输出，但文件记录不完整
4. **Channel 满时的丢弃**：高并发超出 channel 容量时直接丢弃（通过 `Dropped()` 方法暴露计数，日志可观测）

> 对于场景 1，建议生产环境配合外部日志收集（如 sidecar / fluentd）兜底。

