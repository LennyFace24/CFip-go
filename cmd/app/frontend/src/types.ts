/**
 * 前端领域类型。
 *
 * 约定：组件与 composables 只使用小驼峰的领域模型；
 * Go 侧大驼峰的原始形状统一以 `XxxDTO` 命名，只允许出现在 services/api.ts 中。
 */

export type Tab = 'speed' | 'settings'

/** 结果表格排序方式 */
export type SortMode = 'latency' | 'ip' | 'status'

/** 一条测速结果。latency 单位毫秒，小于 0 表示探测失败 */
export interface Row {
  seq: number
  ip: string
  latency: number
  source: string
  /** 机房代码，取自 cf-ray，可能为空 */
  colo: string
  /** 是否通过机房白名单 */
  allowed: boolean
}

/** 测速进度 */
export interface Progress {
  done: number
  /** 达标：通过白名单、探测成功且延迟不超过上限 */
  qualified: number
  /** 超标：探测成功但延迟超过上限 */
  over: number
  /** 请求失败 */
  failed: number
  /** 机房不符：未通过白名单 */
  excluded: number
}

/** 测速配置 */
export interface Config {
  /** 延迟上限（ms） */
  latency: number
  /** 并发数 */
  concurrency: number
  /** 单请求超时（ms） */
  timeout: number
  /** 达标 IP 数量，集满即停 */
  number: number
  /** 机房白名单，空格分隔的 IATA 代码；空串表示不过滤 */
  colo: string
  /** 配置文件磁盘路径，仅用于展示 */
  path: string
}

/** 内置推荐的机房 */
export interface ColoOption {
  code: string
  name: string
}

/** 导入 ip.txt 的结果 */
export interface ImportResult {
  path: string
  content: string
  count: number
}

/** 一次测速结束后的汇总 */
export interface SpeedSummary {
  /** 已探测总数 */
  total: number
  qualified: number
  overLimit: number
  failed: number
  excluded: number
  elapsed: number
  stopped: boolean
  /** 自动写入的日志文件路径 */
  logPath: string
}

/** 结果状态：达标 / 超标 / 失败 / 机房不符 */
export type RowState = 'qualified' | 'over' | 'failed' | 'excluded'

/** Toast 类型 */
export type ToastKind = 'info' | 'success' | 'error'

export interface ToastItem {
  id: number
  text: string
  kind: ToastKind
}

/** 内置网段列表及其来源 */
export interface CidrSource {
  cidrs: string[]
  /** true = 在线拉取成功，false = 回退编译期内置列表 */
  online: boolean
  /** 在线拉取失败原因 */
  error: string
}

/* ---------------- Go 侧原始形状（仅 services/api.ts 可见） ---------------- */

export interface ConfigDTO {
  Latency: number
  Concurrency: number
  Timeout: number
  Number: number
  Colo: string
  Path: string
}

export interface ColoOptionDTO {
  Code: string
  Name: string
}

export interface CidrSourceDTO {
  CIDRs: string[]
  Online: boolean
  Error: string
}

export interface ImportResultDTO {
  Path: string
  Content: string
  Count: number
}

export interface SpeedResultDTO {
  Seq: number
  IP: string
  Latency: number
  Source: string
  Colo: string
  Allowed: boolean
}

export interface SpeedSummaryDTO {
  Total: number
  Qualified: number
  OverLimit: number
  Failed: number
  Excluded: number
  Elapsed: number
  Stopped: boolean
  LogPath: string
}
