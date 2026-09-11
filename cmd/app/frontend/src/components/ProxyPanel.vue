<script setup lang="ts">
import { computed } from 'vue'
import StatTile from './StatTile.vue'
import type { PoolNode, PoolSnapshot } from '../types'

const props = defineProps<{
  snapshot: PoolSnapshot
  /** 当前 IP 来源的候选数量，为 0 时无法启动 */
  ipCount: number
}>()

const emit = defineEmits<{
  start: []
  stop: []
  recheck: []
  export: []
  openDir: []
}>()

interface NodeGroup {
  key: string
  title: string
  tagClass: string
  tagText: string
  nodes: PoolNode[]
  emptyHint: string
}

const scanning = computed(() => props.snapshot.phase === 'scanning')
const hasNodes = computed(() => props.snapshot.primary.length + props.snapshot.backup.length > 0)

const scanPercent = computed(() => {
  const { scanDone, scanTotal } = props.snapshot
  if (!scanTotal) return 0
  return Math.min(100, Math.round((scanDone / scanTotal) * 100))
})

const statusText = computed(() => {
  if (props.snapshot.phase === 'scanning') return '扫描中'
  if (props.snapshot.running) return '运行中'
  return '已停止'
})

const emptyHint = computed(() => {
  const current = props.snapshot
  if (!current.running) return '未启动'
  if (current.phase === 'scanning') return '扫描中，暂无节点'
  return '扫描完成，但没有节点达标'
})

const groups = computed<NodeGroup[]>(() => {
  const current = props.snapshot
  return [
    {
      key: 'primary',
      title: `主选负载均衡池（${current.primary.length} / ${current.primaryTarget}）`,
      tagClass: 'qualified',
      tagText: '主选',
      nodes: current.primary,
      emptyHint: emptyHint.value,
    },
    {
      key: 'backup',
      title: `备用池（${current.backup.length} / ${current.backupTarget}）`,
      tagClass: 'excluded',
      tagText: '备用',
      nodes: current.backup,
      emptyHint: emptyHint.value,
    },
  ]
})

function formatLatency(value: number): string {
  return value >= 0 ? value.toFixed(1) : '—'
}

function formatLoss(value: number): string {
  return `${(value * 100).toFixed(0)}%`
}

function formatTime(ms: number): string {
  if (!ms) return '—'
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<template>
  <section class="card">
    <header class="card-head">
      <div class="card-title">
        <div>
          <h2>代理</h2>
          <p class="card-sub">
            一键完成测速、优选、入池与转发；运行中周期复测，劣化节点先隔离再淘汰并自动补测新 IP
          </p>
        </div>
      </div>
      <span class="chip" :class="snapshot.running ? 'live' : ''">{{ statusText }}</span>
    </header>

    <div class="row-actions">
      <button
        v-if="!snapshot.running"
        class="btn primary"
        :disabled="!ipCount"
        @click="emit('start')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5l11 7-11 7V5z" /></svg>
        启动代理
      </button>
      <button v-else class="btn danger" @click="emit('stop')">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 7h10v10H7z" /></svg>
        停止
      </button>

      <button class="btn ghost" :disabled="!snapshot.running" @click="emit('recheck')">
        立即复测
      </button>
      <button class="btn ghost" :disabled="!hasNodes" @click="emit('export')">导出快照</button>
      <button class="btn ghost" @click="emit('openDir')">日志目录</button>

      <span v-if="!ipCount" class="hint-text">请先在上方设置 IP 来源</span>
    </div>

    <div v-if="scanning" class="progress">
      <div class="progress-bar indeterminate">
        <span :style="{ width: scanPercent + '%' }" />
      </div>
      <span class="progress-text">{{ snapshot.scanDone }} / {{ snapshot.scanTotal }}</span>
    </div>

    <div class="stats">
      <StatTile label="主选" :value="`${snapshot.primary.length} / ${snapshot.primaryTarget}`" tone="ok" />
      <StatTile label="备用" :value="`${snapshot.backup.length} / ${snapshot.backupTarget}`" />
      <StatTile label="已淘汰" :value="snapshot.evictions.length" tone="warn" />
      <StatTile label="活跃连接" :value="snapshot.activeConns" />
      <StatTile label="SOCKS5 监听" :value="snapshot.listenAddr || '—'" mono />
    </div>

    <p v-if="snapshot.listenError" class="warn-text">监听失败：{{ snapshot.listenError }}</p>
    <p v-else-if="snapshot.listenAddr" class="hint-text">
      代理已就绪：把浏览器或应用的 SOCKS5 代理指向 {{ snapshot.listenAddr }}，即可通过优选节点访问
      Cloudflare 上的站点
    </p>

    <template v-for="group in groups" :key="group.key">
      <div class="table-wrap">
        <h3 class="group-title">{{ group.title }}</h3>
        <table>
          <thead>
            <tr>
              <th>分组</th>
              <th>IP</th>
              <th>机房</th>
              <th class="num-col">延迟 (ms)</th>
              <th class="num-col">丢包率</th>
              <th class="num-col">采样</th>
              <th>最后检查</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="node in group.nodes" :key="node.ip">
              <td>
                <span v-if="node.isolated" class="tag over">隔离</span>
                <span v-else class="tag" :class="group.tagClass">{{ group.tagText }}</span>
              </td>
              <td class="mono">{{ node.ip }}</td>
              <td class="mono dim">{{ node.colo || '—' }}</td>
              <td class="num-col mono">{{ formatLatency(node.latency) }}</td>
              <td class="num-col mono">{{ formatLoss(node.lossRate) }}</td>
              <td class="num-col mono dim">{{ node.samples }}</td>
              <td class="dim">{{ formatTime(node.updatedAt) }}</td>
            </tr>
            <tr v-if="!group.nodes.length">
              <td colspan="7" class="empty">{{ group.emptyHint }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <div v-if="snapshot.evictions.length" class="evict-log">
      <h3 class="group-title">最近淘汰</h3>
      <ul>
        <li
          v-for="(item, index) in snapshot.evictions.slice().reverse()"
          :key="`${item.ip}-${item.time}-${index}`"
        >
          <span class="mono">{{ item.ip }}</span>
          <span class="dim">{{ item.reason }}</span>
          <span class="dim">{{ formatTime(item.time) }}</span>
        </li>
      </ul>
    </div>
  </section>
</template>
