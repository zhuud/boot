# boot

本库是 Go 规范的实现示例。编写或修改本库时：

1. 先读下方规范，再读相邻包的现有写法。
2. 行为变化补测试；热路径补 benchmark。
3. 提交前：`gofmt -s -w . && go vet ./... && go test ./... && go test -race ./...`

---

# Go 代码规范（LLM）

## 0. 使用说明

- **适用范围**：人工维护的 Go 库与服务代码。生成代码、syscall/cgo 受生成器或外部约定控制。
- **格式职责**：排版、import 排序交给 `gofmt` / `goimports`；本文约束命名、组织、API、错误、并发、风险治理与测试。
- **冲突优先级（高到低）**：Go 官方规范（Effective Go / Code Review Comments）> 本文 > 当前文件或包的既有写法。
- **设计优先级（高到低）**：清晰 > 简单 > 简洁 > 可维护 > 一致。先选标准库，再考虑第三方。
- **规则强度**：`MUST` 强制；`MUST NOT` 禁止；`SHOULD` 默认遵守，有充分理由才偏离；`MAY` 可选。
- **示例职责**：各章节的 `OK/BAD` 只解释相邻规则，不覆盖规则正文。

模型执行任务时：

1. 先识别任务涉及的规则分类。
2. 生成或修改代码时遵守所有 `MUST` / `MUST NOT`。
3. 偏离 `SHOULD` 时，在结果中说明具体原因。
4. 审查代码时，按“正确性与并发 > API 与错误 > 命名与组织 > 性能与格式”排序问题。
5. 完成后执行第 14 节检查；不要仅凭代码看起来正确就跳过验证。

本仓库是该规范的实现示例。Cursor 编写或审查 Go 时使用 skill `go-code-style`。

### 任务索引

| 任务 | 章节 / 规则 |
|---|---|
| 格式、静态检查、CI | 1 / `TOOL-*` |
| 命名、包名、缩写、`Id` | 2 / `NAME-*` |
| import、声明与文件组织 | 3 / `ORG-*` |
| 中文注释与 godoc | 4 / `DOC-*` |
| error、包装、日志与 panic | 5 / `ERR-*` |
| 接口与 Context | 6 / `ABST-*` |
| goroutine、channel、锁与所有权 | 7 / `CONC-*` |
| 接收者、指针与值 | 8 / `VALUE-*` |
| 构造、配置与数据边界 | 9 / `API-*` |
| 控制流与资源释放 | 10 / `FLOW-*` |
| 测试 | 11 / `TEST-*` |
| 性能与标准库 API | 12 / `PERF-*`、`STDLIB-*` |
| 风险治理、幂等、事务、边界与 `TODO(risk)` | 13 / `RISK-*` |

---

## 1. 工具与格式（TOOL）

- `TOOL-01 MUST` 保存时运行 `goimports`（包含 `gofmt`）。
- `TOOL-02 MUST` 提交前至少运行：

  ```text
  gofmt -s -w .
  go vet ./...
  ```

- `TOOL-03 SHOULD` 全仓统一 `golangci-lint` 配置，至少启用 `govet`、`errcheck`、`staticcheck`、`revive`、`goimports`。
- `TOOL-04 MUST NOT` 为凑行宽破坏可读性。无硬性行宽；不要在缩进变化处或 URL 中间硬折行。
- `TOOL-05 MUST` CI 至少执行以下检查，任一失败都阻止合并：

  ```text
  gofmt -s -w .        # 执行后工作区必须无差异
  go vet ./...
  go test ./...
  go test -race ./...
  ```

- `TOOL-06 MUST` CI 按项目声明的 `GOOS` / `GOARCH` 支持矩阵执行跨平台编译；矩阵至少包含一个非 CI 宿主平台。
- `TOOL-07 MUST` CI 运行受版本控制的关键 benchmark 集，并与已批准基线比较；超过项目阈值的回退必须阻止合并或经明确批准更新基线。

## 2. 命名（NAME）

### 标识符与包

- `NAME-01 MUST` 标识符使用 MixedCaps：`MaxLength`、`maxLength`。不得使用 `MAX_LENGTH`、`max_length`。文件名可用下划线。
- `NAME-02 MUST` 包名小写、单数、一个词；目录最后一段与 `package` 名一致。
- `NAME-03 MUST NOT` 使用空泛包名：`util`、`common`、`misc`、`helper`、`lib`、`api`、`types`。
- `NAME-04 MUST NOT` 在导出名中重复包名：用 `http.Server`，不用 `http.HTTPServer`。
- `NAME-05 SHOULD` 主类型构造函数命名为 `New`，其他构造函数命名为 `NewXxx`。
- `NAME-06 MUST NOT` 无冲突时重命名 import；有冲突时重命名作用域更小的包。

### 缩写、变量与接收者

- `NAME-07 MUST` 缩写保持整体大小写：`ServeHTTP`、`URLPony`、`JSONDecoder`、`XMLAPI`、`GRPC`、`DB`。不得写 `ServeHttp`、`HttpUrl`、`JsonDecoder`。
  `Id` 例外：一律写成 `Id`，不得写成 `ID`。用 `userId`、`requestId`、`UserId`，不用 `userID`、`UserID`。
- `NAME-08 SHOULD` 名字长度与作用域成正比。循环索引和惯用短名可用 `i`、`n`、`ok`、`ctx`、`err`、`buf`、`mu`。领域对象、配置和 Option 即使是局部变量也用完整角色名，见 `NAME-18`。
- `NAME-09 MUST NOT` 在变量名中重复类型或删除字母造缩写：用 `users`、`name`、`Sandbox`，不用 `userSlice`、`nameString`、`Sbx`。
- `NAME-10 MUST` 接收者用类型的 1–2 字母缩写，同一类型保持一致，如 `h`、`w`、`c`；不得使用 `this`、`self`、`me` 或类型全名。接收者不受 `NAME-18` 约束。

### 函数、常量与错误

- `NAME-11 MUST NOT` 给普通 getter 加 `Get`；HTTP GET 语义除外。阻塞或可能失败的读取可用 `Fetch`、`Load`、`Compute`。
- `NAME-12 MUST` 常量使用 MixedCaps 并按角色命名，不按字面值命名。实现细节常量不导出，用完整角色名：`truncatedKey`、`defaultMaxSize`。
- `NAME-13 MUST` 导出枚举值带类型前缀。零值不能碰巧代表有效业务值：用 `Unspecified` / `Unknown`，或从 1 开始。当零值就是该 API 的默认可用状态时（`API-02`），MAY 让 iota 0 表示该默认，并在类型 godoc 写明，例如默认 Text 编码的 `FormatText`。
- `NAME-14 MUST` 错误值命名为 `ErrXxx` / `errXxx`；错误类型命名为 `XxxError` / `xxxError`。
- `NAME-15 MAY` 测试函数用下划线分组：`TestSplitHostPort_EmptyHost`。
- `NAME-16 SHOULD` 从调用方视角选择包名：不要占用 `buf` 等常用短变量；预计会被同时导入的包应避免同名，也不要抢占 `io`、`http`、`json` 等惯用名。
- `NAME-17 SHOULD` 局部变量按当前位置的角色命名，表达“这里是什么”，而不是“数据从哪里来”。
- `NAME-18 MUST` 参数和跨语句局部变量使用完整角色名。不得用 `opts`、`opt`、`cfg`，也不得把领域对象缩成 `r`、`a`，或把下游 handler 叫 `inner`。

  | 角色 | 写法 | 不要 |
  |---|---|---|
  | Functional Option 参数 | `options ...Option`，循环变量 `option` | `opts`、`opt` |
  | 构建描述 | `config` | `cfg`（接收者除外） |
  | 领域对象 | `handler`、`extractor`、`attr`、`record` | 参数里的 `h`、`r`、`a` |
  | 包装下游 | `next` | `inner`、参数里的 `h` |

- `NAME-19 MUST` 单个实体、函数或回调用单数：`extractor`、`dropFunc`。切片、map 用复数：`extractors`。包含多项配置的结构体即使用一个值也叫 `Options` 或 `Config`：`handlerOptions`、`SamplingConfig`。Functional Option 函数类型本身用单数 `Option`，见 `API-04`。
- `NAME-20 SHOULD` 导出配置函数 `WithXxx`；私有构造 `newXxx` / `newXxxHandler`。实现类型不导出：`handlerConfig`、`truncateHandler`。导出回调/谓词用角色名：`DropFunc`、`ErrorFunc`。

### 示例（OK/BAD）

| 规则 | OK | BAD |
|---|---|---|
| `NAME-01` | `maxLength`、`MaxLength` | `max_length`、`MAX_LENGTH` |
| `NAME-04` | `http.Server`、`bufio.Reader` | `http.HTTPServer`、`bufio.BufReader` |
| `NAME-07` | `ServeHTTP`、`userId`、`JSONDecoder` | `ServeHttp`、`userID`、`JsonDecoder` |
| `NAME-09` | `users`、`name`、`Sandbox` | `userSlice`、`nameString`、`Sbx` |
| `NAME-10` | `func (c *Client) Close()` | `func (this *Client) Close()` |
| `NAME-11` | `u.Name()`、`FetchProfile(ctx, id)` | `u.GetName()`、`GetProfile(id)` |
| `NAME-18` | `options ...Option`、`config`、`record` | `opts`、`cfg`、`r` |

```go
// OK: 零值明确表示未指定。
type Format int

const (
	FormatUnspecified Format = iota
	FormatText
	FormatJSON
)

// OK: 零值就是文档化的默认可用状态（API-02），iota 0 可以为 FormatText。
// Format 选择基座编码。零值表示 FormatText。
type Format int

const (
	FormatText Format = iota
	FormatJSON
)

// BAD: 零值碰巧代表有效业务状态，且值名没有类型前缀。
const (
	Text Format = iota
	JSON
)
```

```go
// OK: 包名不抢调用方常用的 buf 变量。
package bufio

// BAD: 调用方很可能同时需要名为 buf 的局部变量。
package buf
```

## 3. 代码组织（ORG）

- `ORG-01 MUST` import 至少分为标准库与其他依赖两组，组间空行。若分为标准库、第三方、本模块三组，必须全仓一致。
- `ORG-02 MUST NOT` 使用点导入；仅黑盒测试为打破循环依赖时例外。
- `ORG-03 MUST NOT` 在普通库代码使用空白导入。它只用于 `main` 或确需副作用的测试。
- `ORG-04 SHOULD` 文件内按调用链和抽象层级排序，不按字母排序：
  1. 常量、类型、配置结构。
  2. 导出 `Option` / `WithXxx`。
  3. 导出构造或入口（`New` / `NewXxx` / `Open`）。
  4. 私有构造与组装（`newXxx`、`newXxxHandler`）。
  5. 接口方法，按接口声明顺序。
  6. 该类型的私有子流程，然后是包级工具函数。
  同一接收者的方法连续，紧跟类型或构造函数，不要被无关函数打断。
- `ORG-05 MUST` 同一接收者的方法连续排列；相关声明可分组，无关声明不得混组。
- `ORG-06 SHOULD` 方法顺序与接口声明顺序一致。`slog.Handler` 固定：`Enabled` → `Handle` → `WithAttrs` → `WithGroup`，然后是私有辅助方法。
- `ORG-07 SHOULD` 有类型状态或语义的逻辑写成方法；无状态、跨类型的纯算法写成包级函数。不用 `buildXxx` / `emptyXxx` 转发一层赋值。
- `ORG-08 MUST NOT` 仅为减少行数抽取只赋值的 `buildXxx`；抽取应对应独立概念。入口里能看完的拼装、映射、环境分支写在调用点。
- `ORG-09 MUST` 同一类结构体字段顺序保持一致。包装/装饰类型：下游（`next`）→ 配置 → 运行时状态。组装用的 `xxxConfig` 按组装或生效顺序排列字段。
- `ORG-10 SHOULD` 包装/装饰类型：`next == nil` 视为程序员错误，可 panic；功能关闭时直接返回 `next`，不包空壳。空的 `WithAttrs` / 空 group 名返回原值。派生用 `clone := *h` 再改 `next` 和本次变化的字段；浅拷贝共享的 slice/map/指针在构造后不得再改。热路径没有改动则传递原值，避免无谓 `Clone`。

### 示例（OK/BAD）

```go
// OK: 标准库与其他依赖分组。
import (
	"context"
	"net/http"

	"golang.org/x/sync/errgroup"
)

// BAD: 无冲突却重命名，且所有 import 混在一组。
import (
	ctxpkg "context"
	"golang.org/x/sync/errgroup"
	"net/http"
)
```

```go
// OK: 抽独立概念；有接收者语义用方法，无状态算法用包级函数。
func truncateUTF8(value string, maxBytes int) (string, bool)
func (h *truncateHandler) truncateAttr(attr slog.Attr) (slog.Attr, bool)

// BAD: 只为少几行把入口拼装提成 buildXxx。
func buildHandler(config *handlerConfig) slog.Handler {
	return newBaseHandler(config)
}
```

```go
// OK: 功能关闭不包空壳；派生 clone 后只改 next。
func newDropHandler(next slog.Handler, dropFunc DropFunc) slog.Handler {
	if next == nil {
		panic("nil log handler")
	}
	if dropFunc == nil {
		return next
	}
	return &dropHandler{next: next, dropFunc: dropFunc}
}

func (h *dropHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.next = h.next.WithAttrs(attrs)
	return &clone
}
```

## 4. 注释（DOC）

- `DOC-01 MUST` 每个导出的顶层名字都有 godoc。注释是中文完整句子，以被描述的名字开头，以中文句号 `。` 结尾。
- `DOC-02 MUST` 包注释紧贴 `package` 行，使用中文。可保留 `Package <name>` 前缀后接中文，或直接以包名开头。`package main` 可用 `xxx 命令 ...`。
- `DOC-03 SHOULD` 注释解释原因、不变量、并发约定、限制、最后一次生效/累加语义和热路径要求，不复述代码或函数名。
- `DOC-04 MUST` 修改行为时同步更新注释；过时注释必须修正或删除。
- `DOC-05 MUST` 注释使用中文，包括包注释、导出 godoc、未导出行为注释和 `TODO`。标识符、包路径、标准库符号、协议名保持原文，不翻译：`Context`、`error`、`Handle`、`slog`。

### 示例（OK/BAD）

```go
// OK: 中文、以导出名开头，说明行为与取消条件。
// Wait 阻塞直到可取 n 个事件，或 ctx 被取消。
func (l *Limiter) Wait(ctx context.Context, n int) error

// BAD: 没以名字开头，只复述“等待”，且使用英文。
// This method waits.
func (l *Limiter) Wait(ctx context.Context, n int) error
```

## 5. 错误（ERR）

### 处理与文案

- `ERR-01 MUST` 每个 error 都要处理、返回，或仅在不变式破坏时 panic。不得用 `_` 静默丢弃；唯一例外是函数已因更早错误返回且无法附加资源关闭错误，见 `FLOW-05`。
- `ERR-02 MUST` 先处理错误并早返回，保持成功路径少缩进；后续仍需变量时，不要把声明藏在 `if` 初始化语句中再加 `else`。
- `ERR-03 MUST` 错误字符串小写开头、句末无标点；专有名词和缩写例外。日志是完整消息，可正常大写。
- `ERR-04 SHOULD` 包装文案使用简短操作名：`fmt.Errorf("open %s: %w", path, err)`；不要堆叠 `failed to`。
- `ERR-05 MUST` 同一错误只处理一次：要么向上返回，要么在边界记录日志、降级或转为领域错误；不得既记录 Error 日志又包装返回。

### 返回与匹配

| 调用方需要匹配 | 文案 | 实现 |
|---|---|---|
| 否 | 静态 | `errors.New(...)` |
| 否 | 动态 | `fmt.Errorf(...)` |
| 是 | 静态 | `var ErrXxx = errors.New(...)`，调用方用 `errors.Is` |
| 是 | 动态 | 自定义错误类型，调用方用 `errors.As` |

- `ERR-06 MUST` 运行时错误向调用方返回。没有新上下文时原样 `return err`；添加上下文时用 `%w` 保留错误链；只有明确要切断匹配时才用 `%v`。
- `ERR-07 MUST` 将导出的哨兵错误和自定义错误类型视为稳定 API，并测试 `errors.Is` / `errors.As` 行为。
- `ERR-08 MUST NOT` 用 `-1`、`""`、`nil` 等带内值表示业务失败；另返回 `error` 或 `ok`。纯变换函数返回自然零值除外。
- `ERR-09 MUST NOT` 因用户输入、配置、环境、依赖或其他运行时失败 panic。panic 仅用于明确的程序员错误，如不可能分支或程序不变式破坏；`MustXxx` 仅用于由程序员控制的启动期常量。
- `ERR-10 MUST NOT` 依赖 `http.Server`、任务调度器或其他框架的 recover 使 panic 成为正常控制流；上层能够恢复不代表库代码可以主动 panic。

### 示例（OK/BAD）

```go
// OK: 错误路径早返回，并用 %w 保留错误链。
config, err := loadConfig(path)
if err != nil {
	return fmt.Errorf("load config %s: %w", path, err)
}
return use(config)

// BAD: 成功路径陷入 else，且 %v 切断错误链。
if config, err := loadConfig(path); err != nil {
	return fmt.Errorf("Load config failed: %v.", err)
} else {
	return use(config)
}
```

```go
// OK: 由调用边界统一记录最终错误。
if err := store.Save(ctx, item); err != nil {
	return fmt.Errorf("save item %q: %w", item.Id, err)
}

// BAD: 同一错误既记录又返回，通常会产生重复日志。
if err := store.Save(ctx, item); err != nil {
	logger.Error("save failed", "error", err)
	return fmt.Errorf("save item %q: %w", item.Id, err)
}
```

```go
// OK: 用 errors.Is 匹配错误链。
if errors.Is(err, ErrNotFound) {
	return nil
}

// BAD: 包装后直接比较会失败。
if err == ErrNotFound {
	return nil
}
```

```go
// OK: 运行时错误返回给调用方。
func Open(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &File{file: f}, nil
}

// BAD: 用户输入或环境错误触发 panic。
func MustOpenUserPath(path string) *File {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	return &File{file: f}
}
```

## 6. 接口与 Context（ABST）

- `ABST-01 MUST` 接口定义在使用方；实现方返回具体类型。
- `ABST-02 MUST NOT` 仅为 mock 在生产包中预先定义接口。测试优先使用真实 API 或局部替身。
- `ABST-03 SHOULD` 接口尽量小；参数接收接口，返回具体类型。
- `ABST-04 MUST NOT` 使用接口指针，如 `*io.Reader`。
- `ABST-05 MAY` 用 `var _ http.Handler = (*Handler)(nil)` 做编译期实现检查。
- `ABST-06 MUST` `context.Context` 是第一个参数且命名为 `ctx`：`func Fetch(ctx context.Context, ...)`。
- `ABST-07 MUST NOT` 把 Context 放进结构体或自造 Context 接口；实现既有外部接口时例外。
- `ABST-08 MUST` 仅 trace、deadline、cancel 等请求范围元数据进入 Context；普通业务数据通过参数或 receiver 传递。
- `ABST-09 MUST` 沿调用链传递现有 Context。仅与请求无关的后台工作使用 `context.Background()`，并用 cancel/timeout 管理退出。
- `ABST-10 MUST` 任何可能阻塞、等待、执行 I/O 或调用远程依赖的操作都接收 `context.Context`，并遵守 `ABST-06` 的首参数约定；实现既有外部接口时例外，但内部调用仍须传递 Context。
- `ABST-11 SHOULD` Context 视为不可变值，同一 `ctx` 可传给多个并发调用。只有需要改变 deadline、cancel 或 request-scoped value 语义时才派生子 Context；派生操作返回 cancel 函数时必须及时调用。

### 示例（OK/BAD）

```go
// OK: 使用方定义最小接口，实现方返回具体类型。
type Clock interface {
	Now() time.Time
}

func NewRealClock() *RealClock { return &RealClock{} }

// BAD: 实现方为 mock 提前定义宽接口并返回接口值。
type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
	Reset()
}

func NewRealClock() Clock { return &RealClock{} }
```

```go
// OK: 可能阻塞的操作把 ctx 放在第一位。
func Fetch(ctx context.Context, id string) (*User, error)

// BAD: 远程调用无法取消，或 ctx 位置不固定。
func Fetch(id string) (*User, error)
func Fetch(id string, ctx context.Context) (*User, error)
```

```go
// OK: Context 沿调用链传递，派生的 cancel 被调用。
child, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
return backend.Fetch(child, id)

// BAD: 请求链中丢弃调用方的取消与 deadline。
return backend.Fetch(context.Background(), id)
```

```go
// OK: 请求数据是显式参数。
func Authorize(ctx context.Context, userId string, action Action) error

// BAD: 普通业务数据隐藏在 Context 中。
ctx = context.WithValue(ctx, "user_id", userId)
```

## 7. 并发与所有权（CONC）

- `CONC-01 SHOULD` 函数默认同步完成后返回；由调用方决定是否启动 goroutine。
- `CONC-02 MUST` 每个 goroutine 都有明确退出条件和等待/取消机制，可用 `errgroup`、`WaitGroup`、`ctx.Done()`；不得 fire-and-forget。
- `CONC-03 MUST NOT` 向已关闭 channel 发送。
- `CONC-04 SHOULD` channel 容量使用 0（同步）或 1（解锁）；更大容量必须能解释其业务含义。
- `CONC-05 MUST NOT` 嵌入 `sync.Mutex`、复制含锁结构体，或仅为“像锁”而保存 Mutex 指针。含锁类型必须用指针接收者。
- `CONC-06 MUST` 多把锁按职责命名，如 `stateMu`、`mapMu`。
- `CONC-07 SHOULD` slice/map 在入库、返回调用方或交给 goroutine 时复制，避免所有权不明和共享底层存储。
- `CONC-08 SHOULD` 共享对象在构造完成后保持不可变。必须支持修改时，要明确所有权、同步策略和可见性保证，并在 godoc 中说明。
- `CONC-09 MUST` 每项对外并发承诺都有针对性测试，并在 `go test -race ./...` 下通过；普通功能测试不能替代并发行为测试。
- `CONC-10 MUST NOT` 无必要提供异步 API。确需异步时，API 必须说明取消、退出、等待和背压语义，且不得把 goroutine 生命周期隐式留给调用方猜测。
- `CONC-11 MUST` `sync.Mutex` 和 `sync.RWMutex` 使用零值并按值存放；不得仅为初始化或“看起来像锁”而改用指针。

### 示例（OK/BAD）

```go
// OK: 同步 API 由调用方决定是否并发。
func (w *Watcher) Run(ctx context.Context) error {
	return w.loop(ctx)
}

// BAD: 隐式启动 goroutine，调用方无法等待或观察错误。
func (w *Watcher) Start() {
	go w.loop(context.Background())
}
```

```go
// OK: goroutine 有统一取消和等待点。
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return consume(ctx) })
g.Go(func() error { return publish(ctx) })
return g.Wait()

// BAD: fire-and-forget，生命周期和错误都丢失。
go consume(ctx)
go publish(ctx)
return nil
```

```go
// OK: 锁使用零值并按值存放，不嵌入。
type Counter struct {
	mu    sync.Mutex
	count int
}

// BAD: 锁被导出，且锁指针增加不必要的初始化状态。
type Counter struct {
	*sync.Mutex
	count int
}
```

```go
// OK: 边界处复制 slice，调用方不能修改内部状态。
func (s *Store) Ids() []string {
	return append([]string(nil), s.ids...)
}

// BAD: 返回共享底层数组。
func (s *Store) Ids() []string {
	return s.ids
}
```

```go
// OK: channel 容量表达“只保留一个待处理信号”。
changed := make(chan struct{}, 1)

// BAD: 任意容量无法解释内存上界与背压语义。
jobs := make(chan Job, 100)
```

## 8. 接收者与值（VALUE）

| 情况 | 选择 |
|---|---|
| 修改 receiver、含 Mutex、大结构体 | `*T` |
| map / func / chan | 值 |
| 小且不可变，如 `time.Time`、基本类型 | 值 |
| 无法确定 | `*T` |

- `VALUE-01 MUST NOT` 同一类型混用值接收者和指针接收者。
- `VALUE-02 MUST NOT` 按值复制方法集在 `*T` 上的类型，如 `bytes.Buffer`。
- `VALUE-03 MUST NOT` 为减少复制而把 `string`、接口值等小固定尺寸值改为指针。

## 9. API 设计（API）

### 构造与配置

- `API-01 MUST` `New` / `NewXxx` / `Open` 返回立即可用的对象，必要时同时返回 error。
- `API-02 SHOULD` 在不掩盖缺失的必需配置或非法业务状态的前提下，让类型零值具有安全、确定且文档化的行为。零值不必代表有效业务状态；枚举遵守 `NAME-13`。无法满足时应提供构造函数，且对零值的操作返回明确错误而不是 panic。
- `API-03 MUST` 长期配置使用 `Config`（随组件组装或存活，如 `handlerConfig`、`tls.Config`）；单次调用配置使用 `Options`（如 `registerOptions`、`sql.TxOptions`）。二者不混用。
- `API-04 SHOULD` 可选参数较多且 API 需平滑扩展时使用 Functional Options。类型名单数 `Option`，对外函数 `WithXxx`。
- `API-05 MUST` Functional Option 的标量/回调采用“最后一次生效”，集合采用“累加”；`nil` option 跳过。有禁用语义时，`nil` 或非正值表示禁用，并在 godoc 写清。循环使用 `for _, option := range options`，不要用 `opts`/`opt`。
- `API-06 MUST NOT` 暴露语义不明的裸布尔或数字参数，如 `Open(addr, false, true)`；改用具名 option 或自定义类型。

### 数据类型与边界

- `API-07 MUST` 时刻使用 `time.Time`，间隔使用 `time.Duration`。序列化边界才使用 RFC3339 或 Unix 秒/毫秒，并立即转换。
- `API-08 SHOULD` 结构体字段较多时使用具名字段；零值字段可省略。全零值优先 `var x T`，需要非 nil 指针时用 `&T{}`。
- `API-09 MUST` 只给序列化字段添加 JSON/YAML tag；内部结构体不要为对齐添加 tag。
- `API-10 SHOULD` 空切片默认声明为 `var s []T`。
- `API-11 MUST NOT` 在 API 语义中区分 nil slice 与非 nil 空 slice；序列化需要 `[]` 或 `null` 时在边界明确处理。
- `API-12 SHOULD` 已知容量时预分配：`make([]T, 0, n)`、`make(map[K]V, n)`。
- `API-13 MUST NOT` 使用 `new([]T)`。
- `API-14 SHOULD NOT` 使用 `init()`；能放在 `main` / `New` 的初始化不要放进 `init`。
- `API-15 MUST NOT` 在 `init` 启动 goroutine；`os.Exit` / `log.Fatal` 只允许在 `main`，库代码返回 error。
- `API-16 MUST NOT` 用可变包级变量承载请求状态；配置通过参数注入。
- `API-17 MUST NOT` 遮蔽内建名：`error`、`string`、`recover`、`new`、`make`、`len`。
- `API-18 MUST` 核心能力由可实例化类型承载。包级全局函数只能作为便利入口并委托给实例，不得承载实例 API 无法访问的核心行为或依赖隐式可变全局状态。
- `API-19 MUST NOT` 用 Functional Options 表达必需参数、必需依赖或调用期数据；它只用于构造阶段的可选配置。必需值使用显式构造参数。
- `API-20 MUST` 构造函数在返回对象前应用并校验全部配置。非法或互相冲突的配置必须在构造阶段返回 error，不得延迟到首次运行时暴露。

### 示例（OK/BAD）

```go
// OK: 核心能力由实例承载，全局入口只委托给零值实例。
type Codec struct{}

func (Codec) Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

func Encode(v any) ([]byte, error) {
	return (Codec{}).Encode(v)
}

// BAD: 核心行为只能通过隐式可变全局对象访问。
var defaultCodec = Codec{}

func SetDefaultCodec(c Codec) {
	defaultCodec = c
}

func Encode(v any) ([]byte, error) {
	return defaultCodec.Encode(v)
}
```

```go
// OK: 必需参数显式传入，Option 只表达可选配置。
client, err := NewClient(endpoint, WithTimeout(2*time.Second))

// BAD: 必需依赖隐藏在 Option 中，漏传后到运行时才失败。
client := NewClient(WithEndpoint(endpoint), WithTransport(transport))
```

```go
// OK: 构造阶段拒绝非法配置。
func NewClient(endpoint string, options ...Option) (*Client, error) {
	config := clientConfig{timeout: 5 * time.Second}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if endpoint == "" {
		return nil, errors.New("endpoint is empty")
	}
	if config.timeout <= 0 {
		return nil, errors.New("timeout must be positive")
	}
	return &Client{endpoint: endpoint, timeout: config.timeout}, nil
}

// BAD: 非法配置延迟到第一次请求时暴露。
func NewClient(endpoint string, options ...Option) *Client {
	return &Client{endpoint: endpoint}
}
```

```go
// OK: 时间单位由类型表达。
func RetryAfter(d time.Duration)

// BAD: 调用方无法从签名确认单位。
func RetryAfter(milliseconds int64)
```

```go
// OK: API 将两种空切片视为等价。
if len(items) == 0 {
	return nil
}

// BAD: 用 nil 状态承载额外业务语义。
if items == nil {
	return ErrNotLoaded
}
```

## 10. 控制流与资源（FLOW）

- `FLOW-01 MUST` 优先早返回或 `continue`，能结束当前分支时不要写 `else`。
- `FLOW-02 SHOULD` 变量靠近首次使用并缩小作用域。
- `FLOW-03 MUST` 类型断言接收 `ok`；只有断言失败代表程序错误时才可直接断言。
- `FLOW-04 SHOULD` 使用 `defer` 清理 `Unlock`、`Close`、`Cancel`。
- `FLOW-05 MUST` 在成功路径检查 `Close` error。仅当函数已因更早错误返回且无法附加 close error 时，才可显式忽略延迟关闭错误。

### 示例（OK/BAD）

```go
// OK: 类型断言显式处理失败。
user, ok := value.(*User)
if !ok {
	return fmt.Errorf("unexpected type %T", value)
}

// BAD: 外部数据类型不符时直接 panic。
user := value.(*User)
```

```go
// OK: 成功路径返回 Close 错误，已有错误优先保留。
func writeFile(path string, data []byte) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close %s: %w", path, closeErr)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// BAD: Close 错误被静默丢弃。
defer f.Close()
```

## 11. 测试（TEST）

- `TEST-01 MUST` 新包和重要行为有测试。多组输入使用表驱动测试与 `t.Run`。
- `TEST-02 SHOULD` 测试表命名为 `tests`，元素为 `tt`，期望字段为 `wantXxx`。
- `TEST-03 MUST` 失败消息包含输入，并按 `got ...; want ...` 顺序输出实际值和期望值。
- `TEST-04 MUST NOT` 在测试表中堆叠 `shouldCallX`、函数字段或多层条件；复杂行为拆成多个 `TestXxx`。只有简单成功/失败分支可用 `wantErr bool`。
- `TEST-05 MUST` Go 1.21 及更早版本中，表驱动子测试配合 `t.Parallel()` 时在循环内执行 `tt := tt`；Go 1.22+ 不需要。
- `TEST-06 SHOULD` 为导出 API 提供可运行 `Example`。
- `TEST-07 MUST` 比较时间、map、浮点和含不稳定字段的结构体时使用语义正确的比较方式；结构体可用 `cmp.Diff` 或逐字段比较。

### 示例（OK/BAD）

```go
// OK: 输入、got、want 都能从失败日志定位。
func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trim", in: " a ", want: "a"},
		{name: "empty", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.in); got != tt.want {
				t.Errorf("Normalize(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

// BAD: 失败时不知道输入，也颠倒 got/want。
if want != got {
	t.Errorf("want %q, got %q", want, got)
}
```

```go
// OK: 并发承诺由真实并发访问覆盖，并配合 go test -race。
func TestCounterConcurrent(t *testing.T) {
	var c Counter
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}
	wg.Wait()
	if got := c.Value(); got != 100 {
		t.Errorf("Value() = %d; want 100", got)
	}
}

// BAD: 只有串行功能测试，无法验证并发承诺。
func TestCounter(t *testing.T) {
	var c Counter
	c.Inc()
}
```

## 12. 性能与标准库（PERF/STDLIB）

- `PERF-01 MUST` 先 profile 再优化；正确性优先。不得仅为“少逃逸”改变接收者。
- `PERF-02 SHOULD` 数字/字符串转换用 `strconv`；循环拼接字符串用 `strings.Builder`。
- `PERF-03 SHOULD` 避免循环内反复转换同一份 `[]byte` / `string`，并在可估算时预分配容量。
- `PERF-04 MUST` 注意 `range` 大数组会复制；热路径改用数组指针或切片。

新 API 应优先对齐标准库：

| 场景 | 形状 |
|---|---|
| 字节流 | `io.Reader` / `ReaderAt` / `Closer` 组合 |
| HTTP | `http.Handler`；中间件 `func(http.Handler) http.Handler` |
| 打开资源 | `Open(...) (*T, error)`；`Close() error` |
| 解析/解码 | `Parse` / `Decode` 返回值与 error |
| 编码/写出 | `Write` / `Encode` 接收 `io.Writer` |
| 可取消等待 | `Wait(ctx) error` |
| 日志 | Go 1.21+ 使用 `log/slog` 结构化日志 |
| 随机密钥 | 使用 `crypto/rand`，不得使用 `math/rand` |

- `STDLIB-01 MUST` `*tls.Config` 交付给客户端、服务端或 transport 后不得再修改；需要变更时调用 `Clone` 并修改副本。其他长期共享配置遵守其包文档规定的复制方式。

### 示例（OK/BAD）

```go
// OK: tls.Config 交付后用 Clone 产生变体。
base := &tls.Config{MinVersion: tls.VersionTLS13}
client := &http.Client{Transport: &http.Transport{TLSClientConfig: base}}

next := base.Clone()
next.ServerName = "api.example.com"

// BAD: transport 可能并发读取时继续修改同一配置。
base.ServerName = "api.example.com"
```

```go
// OK: 明确、低开销的数字转换。
s := strconv.Itoa(n)

// BAD: 热路径用通用格式化完成简单转换。
s := fmt.Sprint(n)
```

```go
// OK: 循环拼接使用 Builder，并按已知大小预分配。
var b strings.Builder
b.Grow(totalLength)
for _, part := range parts {
	b.WriteString(part)
}
result := b.String()

// BAD: 循环中反复创建中间字符串。
var result string
for _, part := range parts {
	result += part
}
```

```go
// OK: 密钥使用密码学安全随机源。
if _, err := cryptorand.Read(key); err != nil {
	return fmt.Errorf("generate key: %w", err)
}

// BAD: math/rand 不适合密钥、令牌或 nonce。
mathrand.Read(key)
```

## 13. 风险治理（RISK）

涉及业务逻辑、配置、关键链路、状态机、迁移、并发、幂等或外部依赖时，按类型逐项评估。不假设网络、时间、顺序或外部依赖天然可靠。

- `RISK-01 MUST` 命中下表类型时采用对应治理手段，落到代码、测试、配置或流程。
- `RISK-02 MUST` 无法当场治理时，紧邻代码写中文 `TODO(risk)`，写明条件、影响、兜底和计划。
- `RISK-03 MUST NOT` 为了更短或更快移除：输入校验、鉴权/租户隔离、注入防护、事务保护、有效错误处理、可观测性。
- `RISK-04 MUST` 交付写明命中项、已落地治理、`TODO(risk)`；没有则写“无额外风险”。

| 类型 | 典型场景 | 治理 |
|---|---|---|
| 并发竞态 | 先读后写、重复触发、多实例抢占 | 原子更新、锁/CAS、去重键、串行化 |
| 幂等重试 | MQ 重投、补偿重跑、客户端超时重试 | 幂等键、唯一索引、Upsert、重复短路 |
| 一致事务 | 跨表部分成功、状态机非法跳转 | 事务边界、状态校验、补偿/对账 |
| 输入边界 | 空值、越界、非法枚举、脏数据 | 前置校验、快速失败、鉴权、租户隔离 |
| 容量性能 | 大批量、热点 key、慢 SQL、N+1 | 批查批写、分页、索引、内存上界 |
| 可用稳定 | 依赖抖动、超时堆积、级联失败 | 超时、限流、熔断、退避、降级 |
| 可观测 | 故障难复现、链路不透明 | 结构化日志、指标、trace、告警 |
| 安全合规 | 敏感泄露、越权、注入 | 脱敏、最小权限、参数化、审计 |

### 示例（OK/BAD）

```go
// OK: 无法当场治理时写明条件、影响、兜底和计划。
// TODO(risk): Publish 失败时本地已扣库存。条件：Save 成功但发件失败。影响：超卖。兜底：对账任务补偿。计划：事务发件箱。

// BAD: TODO 无法评估影响与兜底。
// TODO: handle retry
```

```go
// OK: 输入边界在入口快速失败。
if userId == "" {
	return errors.New("user id is empty")
}

// BAD: 把非法输入留给下游 panic 或脏写。
return store.Save(ctx, item)
```

## 14. 完成检查（CHECK）

- [ ] `goimports` / `gofmt` 与 `go vet ./...` 通过。
- [ ] CI 运行普通测试、race 测试、支持矩阵跨平台编译，并比较 benchmark 基线。
- [ ] 包名具体；导出名不重复包名；缩写整体大小写；`Id` 写成 `Id` 不是 `ID`。
- [ ] 参数用 `options`/`option`、`config`、`record`，不用 `opts`/`cfg`/`r`。
- [ ] 导出符号有中文 godoc（以名字开头、句号结尾）；注释与行为一致。
- [ ] 所有 error 已处理；错误文案合规；同一错误只处理一次。
- [ ] `ctx` 是第一参数且未存入结构体；派生 Context 有明确语义并调用 cancel。
- [ ] 接口位于使用方；实现方返回具体类型。
- [ ] goroutine 可退出且可等待；异步 API 明确取消、等待和背压语义。
- [ ] 锁使用零值、未嵌入、未复制；共享对象默认构造后不可变。
- [ ] 并发承诺有针对性 race 测试；长期共享配置交付后未修改。
- [ ] API 没有语义不明的裸布尔/数字，也没有用带内值表示失败。
- [ ] 核心能力由实例承载；Functional Options 只表达可选配置；非法配置在构造阶段返回。
- [ ] 测试覆盖重要行为；失败信息包含 input / got / want。
- [ ] 性能改动有基准或 profile 依据。
- [ ] 涉及并发、幂等、事务、外部依赖等时已按 RISK 表治理；无法治理的有 `TODO(risk)`。
- [ ] 未为缩短或加速移除校验、鉴权、注入防护、事务、错误处理或可观测性。

## 15. 参考依据

- https://go.dev/doc/effective_go
- https://go.dev/wiki/CodeReviewComments
- https://go.dev/blog/package-names
- https://go.dev/doc/comment
- https://google.github.io/styleguide/go/
- https://github.com/uber-go/guide/blob/master/style.md
- https://www.kubernetes.dev/docs/guide/coding-convention/
