/**
 * 后端绑定门面（Facade）。
 *
 * 这是全应用唯一接触 Wails 生成绑定的地方，负责两件事：
 *   1. 把 Go 的大驼峰字段转换成前端的小驼峰领域模型；
 *   2. 把绑定调用收拢成语义化的 API，便于替换实现与单测。
 *
 * 组件与 composables 不允许直接 import bindings。
 */

import {
  ConfigService,
  IPService,
  LogService,
  SpeedService,
} from '../../bindings/github.com/LennyFace24/CFip-go/cmd/app'
import type {
  CidrSource,
  CidrSourceDTO,
  ColoOption,
  ColoOptionDTO,
  Config,
  ConfigDTO,
  ImportResult,
  ImportResultDTO,
} from '../types'

function toConfig(dto: ConfigDTO): Config {
  return {
    latency: dto.Latency,
    concurrency: dto.Concurrency,
    timeout: dto.Timeout,
    number: dto.Number,
    colo: dto.Colo ?? '',
    path: dto.Path,
  }
}

export const api = {
  config: {
    /** 读取当前配置 */
    async get(): Promise<Config> {
      return toConfig((await ConfigService.Get()) as ConfigDTO)
    },

    /** 校验并写入配置 */
    async save(config: Config): Promise<void> {
      await ConfigService.Save(
        config.latency,
        config.concurrency,
        config.timeout,
        config.number,
        config.colo,
      )
    },

    /** 内置默认配置 */
    async defaults(): Promise<Config> {
      return toConfig((await ConfigService.Defaults()) as ConfigDTO)
    },
  },

  ip: {
    /** 内置网段：后端优先在线拉取，失败时回退编译期内置列表 */
    async builtinCidrs(): Promise<CidrSource> {
      const dto = (await IPService.BuiltinCIDRs()) as CidrSourceDTO
      return {
        cidrs: dto.CIDRs ?? [],
        online: Boolean(dto.Online),
        error: dto.Error ?? '',
      }
    },

    /** 对中国大陆较友好的推荐机房，供界面快捷选择 */
    async recommendedColos(): Promise<ColoOption[]> {
      const list = (await IPService.RecommendedColos()) as ColoOptionDTO[]
      return (list ?? []).map((item) => ({ code: item.Code, name: item.Name }))
    },

    /** 指定网段文本，每行一条；cidrs 为空时返回全部内置网段 */
    async builtinText(cidrs: string[], sample: number): Promise<string> {
      return (await IPService.BuiltinText(cidrs, sample)) as string
    },

    /** 打开系统文件对话框导入 ip.txt */
    async importFile(): Promise<ImportResult> {
      const dto = (await IPService.ImportFile()) as ImportResultDTO
      return { path: dto.Path, content: dto.Content, count: dto.Count }
    },

    /** 统计文本中可测速的 IP 数量 */
    async parse(text: string): Promise<number> {
      return (await IPService.Parse(text)) as number
    },

    /** 写入系统剪贴板，返回是否成功 */
    async copy(text: string): Promise<boolean> {
      return (await IPService.CopyToClipboard(text)) as boolean
    },
  },

  log: {
    /** 日志目录路径 */
    async dir(): Promise<string> {
      return (await LogService.Dir()) as string
    },

    /** 弹出保存对话框导出内容，返回实际写入路径 */
    async export(content: string, filename: string): Promise<string> {
      return (await LogService.Export(content, filename)) as string
    },

    /** 用系统文件管理器打开日志目录 */
    async openDir(): Promise<void> {
      await LogService.OpenDir()
    },
  },

  speed: {
    /** 开始测速，结果通过事件推送 */
    async start(text: string): Promise<void> {
      await SpeedService.Start(text)
    },

    /** 停止测速 */
    async stop(): Promise<void> {
      await SpeedService.Stop()
    },

    /** 是否正在测速 */
    async isRunning(): Promise<boolean> {
      return (await SpeedService.IsRunning()) as boolean
    },
  },
}
