/**
 * 日志导出。
 *
 * 测速结束时后端会自动往日志目录写一份 speed-latest.txt（覆盖上一次）；
 * 这里负责把当前结果按同样格式手动导出到用户指定的位置。
 */

import { api } from '../services/api'
import type { Row } from '../types'
import { useToast } from './useToast'

function qualifiedRows(rows: Row[], limit: number): Row[] {
  return rows.filter((row) => row.latency >= 0 && row.latency <= limit)
}

/** 与后端 WriteSpeedLog 保持一致的排版：达标优先、延迟升序、失败最后 */
function toLog(rows: Row[], limit: number): string {
  const qualifiedSet = new Set(qualifiedRows(rows, limit))
  const sorted = [...rows].sort((a, b) => {
    const qa = qualifiedSet.has(a)
    const qb = qualifiedSet.has(b)
    if (qa !== qb) return qa ? -1 : 1
    if (a.latency < 0) return 1
    if (b.latency < 0) return -1
    return a.latency - b.latency
  })

  const lines = [
    'CFip 测速日志',
    `生成时间: ${new Date().toLocaleString('zh-CN', { hour12: false })}`,
    `延迟上限: ${limit} ms`,
    `探测: ${rows.length}    达标: ${qualifiedSet.size}`,
    '',
    'IP\t延迟(ms)\t状态\t来源',
  ]

  for (const row of sorted) {
    const latency = row.latency >= 0 ? row.latency.toFixed(1) : '-'
    const state = row.latency < 0 ? '失败' : row.latency <= limit ? '达标' : '超标'
    lines.push(`${row.ip}\t${latency}\t${state}\t${row.source}`)
  }
  return lines.join('\n')
}

export function useLog() {
  const { notify } = useToast()

  async function exportRows(rows: Row[], limit: number): Promise<void> {
    if (!rows.length) {
      notify('没有可导出的结果', 'error')
      return
    }
    try {
      const path = await api.log.export(toLog(rows, limit), '')
      notify(`已导出 ${rows.length} 条结果`, 'success')
      console.log('[CFip] 日志已导出到', path)
    } catch (e) {
      notify(`导出失败：${e}`, 'error')
    }
  }

  async function openDir(): Promise<void> {
    try {
      await api.log.openDir()
    } catch (e) {
      notify(`打开日志目录失败：${e}`, 'error')
    }
  }

  return { exportRows, openDir }
}
