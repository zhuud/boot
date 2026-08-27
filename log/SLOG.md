# slog 调用链与分层说明

本文说明标准库 `log/slog` 的对象分层、一次打日志的调用链，以及常用 API 的语义与性能取舍。  
本包（`boot/log`）只构建并装饰 Handler；业务打日志仍走 slog。

依据 Go 1.26 `log/slog` 源码（`logger.go` / `record.go` / `handler.go` / `attr.go`）。

---

## 1. 总览

```
业务 / 包级 API
  slog.Info / logger.InfoContext / LogAttrs / With / WithGroup
        │
        ▼
┌─ Logger ─────────────────────────────────────────┐
│  Enabled → Handler.Enabled                       │
│  NewRecord(Time, Level, Message, PC)             │
│  Add / AddAttrs 填字段                           │
│  Handler.Handle(ctx, record)                     │
└──────────────────────┬───────────────────────────┘
                       ▼
┌─ Handler（接口；JSON/Text 等实现）───────────────┐
│  Enabled / Handle / WithAttrs / WithGroup          │
│  标准实现内部：commonHandler.handle                │
└──────────────────────┬───────────────────────────┘
                       ▼
┌─ commonHandler + handleState（包内，不导出）─────┐
│  序列化 time/level/msg/source + attrs → Writer   │
└──────────────────────────────────────────────────┘
```

数据对象关系：

- **Attr**：一个字段（`Key` + `Value`）
- **Record**：一次日志事件（元数据 + 若干 Attr）
- **Logger**：门面，组 Record 并交给 Handler
- **Handler**：是否输出、写成什么格式
- **handleState**：内置 JSON/Text Handler **单次写出**时的临时状态（不导出）

本包插入点：在 Text/JSON Handler 外包策略装饰器，
`NewHandler` 只返回标准 `slog.Handler`。

```
slog.Logger
  → attrGroupHandler   （本包：WithAttrGroup）
  → contextHandler     （本包：ContextExtractor / ContextWithAttrs）
  → redactHandler      （本包：WithRedactKey）
  → dropHandler        （本包：WithDropFunc / DropFunc）
  → samplingHandler    （本包：WithSampling / SamplingConfig）
  → truncateHandler    （本包：WithTruncate）
  → errorHandler       （本包：WithErrorFunc）
  → Text/JSONHandler   （本包按配置创建；source 由 WithSource 控制，不暴露 ReplaceAttr）
      → commonHandler + handleState（标准库内部）
```

Go 1.26 多路输出由标准库 `slog.NewMultiHandler` 完成：`Enabled` 为任意子 Handler 启用即 true，`Handle` / `WithAttrs` / `WithGroup` 广播到各启用子 Handler。每路可有不同格式、级别和 Writer；本包不自研 MultiHandler。

---

## 2. 曝光面：谁给谁用

```
┌──────────────────────────────────────────────────┐
│ 业务日常只用                                      │
│   Logger + Attr 构造函数 + SetDefault / New*Handler│
│   （以及本包 NewHandler / Context*）               │
└──────────────────────────┬───────────────────────┘
                           │
┌──────────────────────────▼───────────────────────┐
│ 写自定义 Handler 才碰                             │
│   Handler 接口 + Record + Attr / Value            │
└──────────────────────────┬───────────────────────┘
                           │
┌──────────────────────────▼───────────────────────┐
│ 标准库内部，不导出                                │
│   commonHandler、handleState、argsToAttr、        │
│   defaultHandler、buffer …                        │
└──────────────────────────────────────────────────┘
```

---

## 3. 各层对象

### 3.1 Attr / Value（字段）

一条结构化字段。`Value` 可表示常用标量且多数情况少分配；实现 `LogValuer` 可延迟求值。

| API | 作用 | 是否导出 |
|-----|------|----------|
| `String` / `Int` / `Bool` / `Any` / `Group` / `Time` … | 构造 Attr | 是 |
| `Attr.Equal` / `String` | 比较、调试 | 是 |
| `Value.Resolve` | 展开 `LogValuer` | 是 |
| `argsToAttr` | 把 `...any` 吃成一个 Attr | **否**（包内） |

`Info("m", args...)` 经 `Record.Add` → `argsToAttr` 变成 Attr；`LogAttrs` 跳过解析，直接用 Attr。

---

### 3.2 Record（一条日志事件）

一次 `Info`/`Log` 的中间表示，交给 `Handler.Handle`。

| 字段 | 作用 |
|------|------|
| `Time` / `Message` / `Level` | 何时、说什么、级别 |
| `PC` | `runtime.Callers` 结果，用于 `Source()`（文件:行） |
| `front[5]` + `nFront` | 栈上最多 5 个 Attr（常见路径少分配） |
| `back` | 超出后的堆上尾部 |

| API | 作用 | 是否导出 |
|-----|------|----------|
| `NewRecord` | 建空 attrs 的 Record（自定义桥接） | 是 |
| `Clone` | `Clip(back)`，断开共享，两边可各自改 | 是 |
| `Add` / `AddAttrs` | 追加字段；跳过空 group | 是 |
| `Attrs` / `NumAttrs` | 遍历 | 是 |
| `Source()` | 用 PC 解析 function/file/line | 是 |

注意：拷贝 Record 会共享 `back` 底层数组；要改拷贝必须先 `Clone()`。业务一般不直接碰 Record，除非实现 Handler。

---

### 3.3 Logger（业务主入口）

薄壳：持有一个 `Handler`，负责组 Record 并下发。

| API | 作用 | 内部落到 |
|-----|------|----------|
| `New` / `Default` / `SetDefault` | 创建 / 读 / 换全局默认 | `atomic.Pointer[Logger]` |
| `Debug`/`Info`/`Warn`/`Error` + `*Context` | 打日志 | `log` → Record → `Handle` |
| `Log` / `LogAttrs` | 任意级别；后者只收 Attr | 同上 |
| `With` | 固定字段挂到后续每条日志 | `Handler.WithAttrs` |
| `WithGroup` | 后续字段加命名空间 | `Handler.WithGroup` |
| `Enabled` / `Handler` | 查询 | 委托 Handler |

包级 `slog.Info` = `Default().Info`。  
不导出：`clone`、内部 `log`/`logAttrs`（抓 PC、填 Record）。

`SetDefault` 还会在默认 Handler **不是** `*defaultHandler` 时，把老 `log` 包的输出桥到 slog（`handlerWriter`），避免与 defaultHandler 死锁。

---

### 3.4 Handler（扩展点 / 写出后端）

真正决定「出不出、写成什么样」。

| 接口方法 | 作用 |
|----------|------|
| `Enabled(ctx, level)` | 级别 / ctx 门控 |
| `Handle(ctx, record)` | 消费 Record 并写出 |
| `WithAttrs(attrs)` | 派生带固定字段的 Handler |
| `WithGroup(name)` | 派生带 group 路径的 Handler |

标准库暴露：`NewJSONHandler` / `NewTextHandler`、`HandlerOptions`（`Level`、`AddSource`、`ReplaceAttr`）。

业务通常调 `logger.With`，不必直接调 Handler 的 `WithAttrs`。标准库也暴露
`Logger.WithGroup`，但本包的业务使用约束禁止调用；实现自定义 Handler 时仍须
完整实现上述四个方法。

`Handle` 约定要点（实现者）：

- `Time`/`PC` 为零则忽略
- Attr 的 Value 应 `Resolve`
- 空 key+空 value 的 Attr 忽略；空 group 忽略；空 key 的 group 内联其子 Attr
- `Logger` 丢弃 `Handle` 返回的 error

---

### 3.5 commonHandler / handleState（仅内置 JSON/Text，不导出）

| 对象 | 生命周期 | 作用 |
|------|----------|------|
| `commonHandler` | 随 Handler 长期存活 | `json`/`opts`/`Writer`、预格式化的 `preformattedAttrs`、`groups`；`handle`/`withAttrs`/`withGroup` |
| `handleState` | 单次 `handle` 或单次 `withAttrs` | 写缓冲、分隔符、key 前缀、`ReplaceAttr` 用的 group 栈；`appendAttr`/`openGroup`/… |

关系：`commonHandler` = 打印机配置 + 已排版页眉；`handleState` = 这一页的稿纸。  
`withAttrs` 时也会临时建 state，把字段**预编码**进 `preformattedAttrs`，之后每条日志直接拷这段字节。

---

## 4. 一次打日志逐步（`InfoContext`）

```go
logger.InfoContext(ctx, "ok", "id", 1)
```

1. `Enabled(ctx, slog.LevelInfo)` → `Handler.Enabled`
2. `NewRecord(now, slog.LevelInfo, "ok", pc)`（`runtime.Callers`）
3. `Record.Add("id", 1)` → `argsToAttr` → Attr `id=1`（先填 `front`）
4. `Handler.Handle(ctx, record)`
5. 若为 JSON/Text：`commonHandler.handle` → `newHandleState` → 写 time/level/msg/(source) → 写预格式化 attrs + Record attrs → `\n` → 加锁写 `Writer` → `free`

经本包 `NewHandler` 时，在步骤 4 进入基座 Handler 前还会：

1. `attrGroupHandler`：显式配置后把本次调用属性收入指定 group  
2. `contextHandler`：跑 ContextExtractor，把 attrs 追加到 Record，不去重  
3. `redactHandler`：按叶子 key 或点分组路径脱敏  
4. `dropHandler`：执行当前 `DropFunc`，返回 true 即丢弃  
5. `samplingHandler`：显式配置后对精确 Warn/Error 的 message 固定桶采样；放行记录附累计 `log.suppressed`  
6. `truncateHandler`：UTF-8 安全截断 message / string / `[]byte`，必要时加 `log.truncated=true`  
7. `errorHandler`：仅基座 `Handle` 返回错误或 panic 时回调；外层装饰器 panic 不会被恢复；panic 转为带堆栈的 error（`slog.Logger` 本身会忽略返回错误）

基座 Text/JSON Handler 在 `WithSource(true)` 时附加 `source`。

因此：请求级字段必须 `ContextWithAttrs` + `*Context`；且 Logger 必须经本包组装，裸 `slog.New(JSONHandler)` 不会读 context attrs。

`ContextWithAttrs` 和 `AttrsFromContext` 在切片边界采用浅拷贝。属性值引用的
可变对象不在拷贝范围内。同名顶层 key（多次 `ContextWithAttrs`、多个 extractor、
本次调用、`logger.With(...)`）均按 slog 原始顺序写出，不由 Context 层去重。

`WithAttrs` / `WithGroup` 会在各策略 Handler 中传播：Redact 会处理固定 attrs，Truncate 会截断固定 attrs，Sampling / ErrorFunc 共享同一状态或回调。`DropFunc` 读取 Context 注入和本次调用的 attrs，不包含 `logger.With(...)` 预绑定的 attrs。

---

## 5. With / WithGroup / Group

### 5.1 `With`：固定字段挂在 Logger 上

```go
l := logger.With(slog.String("service", "api"))
l.Info("up")   // 每条都带 service=api
```

适合进程级或请求入口级稳定字段（`service.name`、`version`）。

### 5.2 `WithGroup`：标准语义与本包约束

```go
logger.WithGroup("request").Info("ok", "id", "r1", "path", "/")
// JSON: {"msg":"ok","request":{"id":"r1","path":"/"}}
```

等价于**这一行**的：

```go
logger.Info("ok", slog.Group("request",
    slog.String("id", "r1"),
    slog.String("path", "/"),
))
```

这是标准库 `WithGroup` 的语义：可以复用多行，而 `slog.Group` 只作用于当前
调用。`WithSource` 写出的 `source` 与 `time` / `level` / `msg` 一样留在根上。
`log.truncated`、`log.suppressed` 仍会进入当前 group。调用参数分组用
`WithAttrGroup`，单次嵌套用行内 `slog.Group`。

叠层：

```go
logger.WithGroup("db").WithGroup("query").Info("slow", "ms", 80)
// "db":{"query":{"ms":80}}
```

与 `With` 组合时**顺序有意义**：

```go
logger.With(slog.String("service", "api")).WithGroup("req")
// service 在顶层；之后字段在 req 下

logger.WithGroup("req").With(slog.String("id", "r1"))
// id 在 req 下
```

业务需要分组时统一使用行内 Group：

```go
logger.Info("ok",
    slog.Group("request", slog.String("id", "r1")),
    slog.Group("resp", slog.Int("code", 0)),
)
```

`WithGroup("")` 为空操作，返回原 Logger。

| 场景 | 选 |
|------|-----|
| 多行共享同一对象前缀 | 每条记录使用同名 `slog.Group` |
| 单行嵌套 / 同级多对象 | `slog.Group` |
| 进程级顶层字段 | `With`（通常不进 group） |
| 请求级关联字段（跨函数） | 本包 `ContextWithAttrs` + `*Context` |

### 5.3 `With(kv)` vs `With(slog.String(...))`

```go
logger.With("service", "api")                 // → Any → AnyValue 再收成 string
logger.With(slog.String("service", "api"))    // → 已是 KindString 的 Attr
```

语义相同。`With` 还会 clone Handler + 预格式化，**解析 args 的差异可忽略**。  
热路径更值得用 `LogAttrs`（跳过 `argsToAttr`），而不是纠结 `With` 的两种写法。

---

## 6. 性能要点

| 做法 | 说明 |
|------|------|
| 热路径 `LogAttrs` | 避免每次 tokenize `...any` |
| `With` | 适合入口做一次；不要每条日志都派生 Logger |
| `front[5]` | 字段少时少堆分配 |
| `WithAttrs` 预格式化 | 固定字段只编码一次 |
| `ReplaceAttr` / 复杂 ContextExtractor | 每条日志都跑，热路径慎用重逻辑 |

`logger.With("k","v")` 与 `With(slog.String(...))` 在 `With` 场景下差距可忽略。

---

## 7. 与本包的关系（速查）

| 需求 | 用法 |
|------|------|
| 建带装饰的 Logger | `slog.New(log.NewHandler(...))` |
| 全局默认 | `log.SetSlogDefault(service, env, options...)` |
| 请求级字段 | `ctx = log.ContextWithAttrs(ctx, ...)` + `InfoContext` |
| Trace / Span | `log.WithContextExtractor(otel.TraceAttrsFromContext)`（`log/extractor/otel`，不随 `SetSlogDefault` 默认启用） |
| 脱敏 | `log.WithRedactKey("password")` |
| 丢弃 | `log.WithDropFunc(fn)`，`fn` 返回 true 时丢弃 |
| 调用参数收入 group | `log.WithAttrGroup("attrs")` |
| source | `SetSlogDefault` 默认打开；`NewHandler` 需 `log.WithSource(true)` |
| Warn/Error 采样 | `SetSlogDefault` 默认 1s/1000/100；`NewHandler` 需 `log.WithSampling(...)` |
| 输出错误观测 | 默认只 recover；`log.WithErrorFunc(fn)` 观测 |
| 多路输出 | `slog.NewMultiHandler(...)`（Go 1.26） |
| 文件轮转 | `log/file.NewWriter` + `lifecycle.RegisterCloser` |
| 改字段名 / 时间格式 | 自行 `slog.NewJSONHandler` / `NewTextHandler` 的 `HandlerOptions.ReplaceAttr`，不要经 `NewHandler` |
| 自定义从 ctx 抽字段 | `log.WithContextExtractor(...)`（默认读取 `ContextWithAttrs` 注入的属性） |

本包不提供包级 `Info`/`Debug`；统一用 `*slog.Logger` / `slog`。
