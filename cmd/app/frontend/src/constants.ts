/** 全局常量：所有魔法数字集中在这里，便于统一调整 */

/** 默认测速参数（后端 config.DefaultConfig 的镜像，首屏先用它渲染） */
export const DEFAULT_CONFIG = {
  latency: 500,
  concurrency: 16,
  timeout: 500,
  number: 20,
} as const

/** 网段采样数默认值与取值范围 */
export const DEFAULT_SAMPLE = 5
export const MIN_SAMPLE = 1
export const MAX_SAMPLE = 2000

/** 输入 IP 文本后重新统计数量的防抖时长 */
export const PARSE_DEBOUNCE_MS = 250

/** Toast 默认停留时长 */
export const TOAST_DURATION_MS = 2600

/** 结果表格排序方式 */
export const SORT_MODES = [
  { key: 'latency', label: '延迟' },
  { key: 'ip', label: 'IP' },
  { key: 'status', label: '状态' },
] as const
