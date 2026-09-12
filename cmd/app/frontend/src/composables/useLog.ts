/**
 * 日志导出。
 *
 * 测速路径下后端会自动写 speed-latest.txt；代理长期运行没有明确终点，
 * 因此这里提供手动导出当前 IP 池快照的能力。
 */

import { api } from '../services/api'
import type { PoolNode, PoolSnapshot } from '../types'
import { useToast } from './useToast'

function formatLatency(value: number): string {
  return value >= 0 ? value.toFixed(1) : '—'
}

function formatTime(ms: number): string {
  if (!ms) return '—'
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function nodeLines(nodes: PoolNode[], group: string): string[] {
  return nodes.map((node) =>
    [
      group,
      node.ip,
      node.colo || '-',
      formatLatency(node.latency),
      `${(node.lossRate * 100).toFixed(0)}%`,
      String(node.samples),
      formatTime(node.updatedAt),
      node.isolated ? '隔离' : '正常',
    ].join('\t'),
  )
}

/** 把池快照排成便于直接阅读的文本 */
export function poolSnapshotText(snapshot: PoolSnapshot): string {
  const lines = [
    'CFip IP 池快照',
    `生成时间: ${new Date().toLocaleString('zh-CN', { hour12: false })}`,
    `阶段: ${snapshot.phase}`,
    `监听: ${snapshot.listenAddr || '未监听'}`,
    `候选: ${snapshot.scanDone} / ${snapshot.scanTotal}`,
    `主选: ${snapshot.primary.length} / ${snapshot.primaryTarget}` +
      `    备用: ${snapshot.backup.length} / ${snapshot.backupTarget}` +
      `    活跃连接: ${snapshot.activeConns}`,
    '',
    '分组\tIP\t机房\t延迟(ms)\t丢包率\t采样\t最后检查\t状态',
    ...nodeLines(snapshot.primary, '主选'),
    ...nodeLines(snapshot.backup, '备用'),
  ]

  if (snapshot.evictions.length) {
    lines.push('', '最近淘汰（原因\t时间）')
    for (const item of snapshot.evictions) {
      lines.push(`${item.ip}\t${item.reason}\t${formatTime(item.time)}`)
    }
  }
  return lines.join('\n')
}

export function useLog() {
  const { notify } = useToast()

  async function exportPool(snapshot: PoolSnapshot): Promise<void> {
    if (!snapshot.primary.length && !snapshot.backup.length) {
      notify('池中还没有节点', 'error')
      return
    }
    try {
      await api.log.export(poolSnapshotText(snapshot), '')
      notify('已导出 IP 池快照', 'success')
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

  return { exportPool, openDir }
}
