/**
 * 测速状态。
 *
 * 后端通过事件逐条推送结果，这里负责订阅事件、维护结果列表与派生统计。
 * 「达标」= 探测成功且延迟不超过本次采用的延迟上限。
 */

import { computed, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { api } from '../services/api'
import { DEFAULT_CONFIG } from '../constants'
import type { Progress, Row, SpeedResultDTO, SpeedSummaryDTO } from '../types'
import { useToast } from './useToast'

const EVENT_RESULT = 'speed:result'
const EVENT_DONE = 'speed:done'

function emptyProgress(): Progress {
  return { done: 0, qualified: 0, over: 0, failed: 0, excluded: 0 }
}

export function useSpeedTest() {
  const { notify } = useToast()

  const rows = ref<Row[]>([])
  const running = ref(false)
  const progress = ref<Progress>(emptyProgress())
  const summary = ref('')
  /** 后端自动写入的日志文件路径 */
  const lastLogPath = ref('')
  /** 本次测速采用的延迟上限（毫秒），用于判定达标 */
  const latencyLimit = ref<number>(DEFAULT_CONFIG.latency)

  // 达标必须通过机房白名单
  const qualified = computed(() =>
    rows.value.filter(
      (row) => row.allowed && row.latency >= 0 && row.latency <= latencyLimit.value,
    ),
  )

  const avgLatency = computed(() => {
    if (!qualified.value.length) return '—'
    const total = qualified.value.reduce((acc, row) => acc + row.latency, 0)
    return (total / qualified.value.length).toFixed(1)
  })

  const bestIp = computed(() => {
    if (!qualified.value.length) return '—'
    return qualified.value.reduce((a, b) => (a.latency <= b.latency ? a : b)).ip
  })

  function subscribe(): void {
    Events.On(EVENT_RESULT, (event: { data: SpeedResultDTO }) => {
      const r = event.data
      rows.value = [
        ...rows.value,
        {
          seq: r.Seq,
          ip: r.IP,
          latency: r.Latency,
          source: r.Source,
          colo: r.Colo,
          allowed: r.Allowed,
        },
      ]

      const excluded = !r.Allowed
      const failed = !excluded && r.Latency < 0
      const over = !excluded && !failed && r.Latency > latencyLimit.value
      const p = progress.value
      progress.value = {
        done: p.done + 1,
        qualified: p.qualified + (!excluded && !failed && !over ? 1 : 0),
        over: p.over + (over ? 1 : 0),
        failed: p.failed + (failed ? 1 : 0),
        excluded: p.excluded + (excluded ? 1 : 0),
      }
    })

    Events.On(EVENT_DONE, (event: { data: SpeedSummaryDTO }) => {
      const s = event.data
      running.value = false
      lastLogPath.value = s.LogPath
      summary.value =
        `${s.Stopped ? '已提前结束' : '全部跑完'} · 达标 ${s.Qualified} · 超标 ${s.OverLimit}` +
        ` · 失败 ${s.Failed} · 机房不符 ${s.Excluded} · 耗时 ${s.Elapsed} ms`
    })
  }

  async function start(text: string, latencyLimitMs: number): Promise<void> {
    if (running.value) return
    if (!text.trim()) {
      notify('请先导入 ip.txt 或加载内置网段', 'error')
      return
    }
    latencyLimit.value = latencyLimitMs > 0 ? latencyLimitMs : DEFAULT_CONFIG.latency
    rows.value = []
    progress.value = emptyProgress()
    summary.value = ''
    lastLogPath.value = ''
    running.value = true
    try {
      await api.speed.start(text)
    } catch (e) {
      running.value = false
      notify(`启动失败：${e}`, 'error')
    }
  }

  async function stop(): Promise<void> {
    try {
      await api.speed.stop()
    } catch (e) {
      notify(`停止失败：${e}`, 'error')
    }
  }

  /** 复制达标的 IP，按延迟升序取前 count 个 */
  async function copyBest(count: number): Promise<void> {
    const best = [...qualified.value]
      .sort((a, b) => a.latency - b.latency)
      .slice(0, Math.max(count, 0))
    if (!best.length) {
      notify('还没有达标的测速结果', 'error')
      return
    }
    await copy(best.map((row) => row.ip).join('\n'), `已复制 ${best.length} 个优选 IP`)
  }

  async function copyOne(ip: string): Promise<void> {
    await copy(ip, `已复制 ${ip}`)
  }

  async function copy(content: string, okMessage: string): Promise<void> {
    try {
      const ok = await api.ip.copy(content)
      notify(ok ? okMessage : '复制失败', ok ? 'success' : 'error')
    } catch (e) {
      notify(`复制失败：${e}`, 'error')
    }
  }

  return {
    rows,
    running,
    progress,
    summary,
    lastLogPath,
    latencyLimit,
    qualified,
    avgLatency,
    bestIp,
    subscribe,
    start,
    stop,
    copyBest,
    copyOne,
  }
}
