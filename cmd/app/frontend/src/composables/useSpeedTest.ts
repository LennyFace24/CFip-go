/**
 * 测速状态。
 *
 * 后端通过事件逐条推送结果，这里负责订阅事件、维护结果列表与派生统计。
 */

import { computed, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { api } from '../services/api'
import type { Progress, Row, SpeedResultDTO, SpeedSummaryDTO } from '../types'
import { useToast } from './useToast'

const EVENT_RESULT = 'speed:result'
const EVENT_DONE = 'speed:done'

function emptyProgress(): Progress {
  return { done: 0, success: 0, failed: 0 }
}

export function useSpeedTest() {
  const { notify } = useToast()

  const rows = ref<Row[]>([])
  const running = ref(false)
  const progress = ref<Progress>(emptyProgress())
  const summary = ref('')

  const succeeded = computed(() => rows.value.filter((row) => row.latency >= 0))

  const avgLatency = computed(() => {
    if (!succeeded.value.length) return '—'
    const total = succeeded.value.reduce((acc, row) => acc + row.latency, 0)
    return (total / succeeded.value.length).toFixed(1)
  })

  const bestIp = computed(() => {
    if (!succeeded.value.length) return '—'
    return succeeded.value.reduce((a, b) => (a.latency <= b.latency ? a : b)).ip
  })

  function subscribe(): void {
    Events.On(EVENT_RESULT, (event: { data: SpeedResultDTO }) => {
      const r = event.data
      rows.value = [...rows.value, { seq: r.Seq, ip: r.IP, latency: r.Latency, source: r.Source }]
      progress.value = {
        done: progress.value.done + 1,
        success: progress.value.success + (r.Latency >= 0 ? 1 : 0),
        failed: progress.value.failed + (r.Latency >= 0 ? 0 : 1),
      }
    })

    Events.On(EVENT_DONE, (event: { data: SpeedSummaryDTO }) => {
      const s = event.data
      running.value = false
      summary.value =
        `${s.Stopped ? '已提前结束' : '全部跑完'} · 成功 ${s.Success} · 失败 ${s.Failed} · 耗时 ${s.Elapsed} ms`
    })
  }

  async function start(text: string): Promise<void> {
    if (running.value) return
    if (!text.trim()) {
      notify('请先导入 ip.txt 或加载内置网段', 'error')
      return
    }
    rows.value = []
    progress.value = emptyProgress()
    summary.value = ''
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

  async function copyBest(limit: number): Promise<void> {
    const best = [...succeeded.value]
      .sort((a, b) => a.latency - b.latency)
      .slice(0, Math.max(limit, 0))
    if (!best.length) {
      notify('还没有成功的测速结果', 'error')
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
    succeeded,
    avgLatency,
    bestIp,
    subscribe,
    start,
    stop,
    copyBest,
    copyOne,
  }
}
