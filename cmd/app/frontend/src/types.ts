/**
 * 前端领域类型。
 *
 * 约定：组件与 composables 只使用小驼峰的领域模型；
 * Go 侧大驼峰的原始形状统一以 `XxxDTO` 命名，只允许出现在 services/api.ts 中。
 */

export type Tab = 'proxy' | 'settings'

/** 代理所处的阶段 */
export type PoolPhase = 'idle' | 'scanning' | 'ready'

/** IP 池中的一个节点 */
export interface PoolNode {
  ip: string
  colo: string
  /** 最近一轮采样的平均延迟（ms） */
  latency: number
  /** EWMA 丢包率，0~1 */
  lossRate: number
  /** 已累积的检查轮数 */
  samples: number
  /** 连续「整轮失败」次数 */
  failStreak: number
  /** 隔离观察中：仍在池内，但不参与流量分发 */
  isolated: boolean
  /** 最后更新时间，Unix 毫秒 */
  updatedAt: number
}

/** 一条淘汰记录 */
export interface EvictionRecord {
  ip: string
  reason: string
  time: number
}

/** IP 池快照 */
export interface PoolSnapshot {
  running: boolean
  phase: PoolPhase
  /** 候选 IP 总数 */
  scanTotal: number
  /** 已探测数量 */
  scanDone: number
  primaryTarget: number
  backupTarget: number
  /** 本地 SOCKS5 监听地址，空串表示未监听 */
  listenAddr: string
  /** 监听失败原因 */
  listenError: string
  /** 当前正在转发的连接数 */
  activeConns: number
  primary: PoolNode[]
  backup: PoolNode[]
  evictions: EvictionRecord[]
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

  /** 主选节点数，承载实际流量 */
  primarySize: number
  /** 备用节点数，主选空缺时递补 */
  backupSize: number
  /** 节点被淘汰后的冷却时长（秒） */
  cooldown: number

  /** 健康检查周期（秒） */
  healthInterval: number
  /** 每次健康检查对每个节点的采样次数 */
  pingTimes: number
  /** 同一次检查内相邻采样的间隔（ms） */
  pingGap: number
  /** 丢包率上限，大于 0 且不超过 1 */
  lossLimit: number

  /** 本地 SOCKS5 监听地址 */
  proxyListen: string

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

/** 内置网段列表及其来源 */
export interface CidrSource {
  cidrs: string[]
  /** true = 在线拉取成功，false = 回退编译期内置列表 */
  online: boolean
  /** 在线拉取失败原因 */
  error: string
}

/** Toast 类型 */
export type ToastKind = 'info' | 'success' | 'error'

export interface ToastItem {
  id: number
  text: string
  kind: ToastKind
}

/* ---------------- Go 侧原始形状（仅 services/api.ts 可见） ---------------- */

export interface ConfigDTO {
  Latency: number
  Concurrency: number
  Timeout: number
  Number: number
  Colo: string

  PrimarySize: number
  BackupSize: number
  Cooldown: number

  HealthInterval: number
  PingTimes: number
  PingGap: number
  LossLimit: number

  ProxyListen: string

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

export interface PoolNodeDTO {
  IP: string
  Colo: string
  Latency: number
  LossRate: number
  Samples: number
  FailStreak: number
  Isolated: boolean
  UpdatedAt: number
}

export interface EvictionRecordDTO {
  IP: string
  Reason: string
  Time: number
}

// Go 的切片可以是 nil，绑定生成的类型因此带 | null
export interface PoolSnapshotDTO {
  Running: boolean
  Phase: string
  ScanTotal: number
  ScanDone: number
  PrimaryTarget: number
  BackupTarget: number
  ListenAddr: string
  ListenError: string
  ActiveConns: number
  Primary: PoolNodeDTO[] | null
  Backup: PoolNodeDTO[] | null
  Evictions: EvictionRecordDTO[] | null
}
