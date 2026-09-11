<script setup lang="ts">
import { computed } from 'vue'
import StatTile from './StatTile.vue'
import type { PoolSnapshot } from '../types'

const props = defineProps<{
  snapshot: PoolSnapshot
  /** 当前 IP 来源的候选数量，为 0 时无法启动 */
  ipCount: number
  starting: boolean
}>()

const emit = defineEmits<{
  start: []
  stop: []
  recheck: []
  export: []
  openDir: []
}>()

const scanning = computed(() => props.snapshot.phase === 'scanning')
const hasNodes = computed(
  () => props.snapshot.primary.length + props.snapshot.backup.length > 0,
)

const scanPercent = computed(() => {
  const { scanDone, scanTotal } = props.snapshot
  if (!scanTotal) return 0
  return Math.min(100, Math.round((scanDone / scanTotal) * 100))
})

const total = computed(
  () => props.snapshot.primary.length + props.snapshot.backup.length,
)

const statusText = computed(() => {
  if (props.snapshot.phase === 'scanning') return '扫描中'
  if (props.snapshot.running) return '运行中'
  return '已停止'
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
        <svg class="card-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M18 16.1a3 3 0 0 0-2.1.9l-6.1-3.6a3.1 3.1 0 0 0 0-2.8l6.1-3.6a3 3 0 1 0-1-2.1c0 .3 0 .5.1.8L8.9 9.3a3 3 0 1 0 0 5.4l6.1 3.6c0 .2-.1.5-.1.7a3 3 0 1 0 3-3z"
          />
        </svg>
        <div>
          <h2>代理</h2>
          <p class="card-sub">
            一键完成测速、优选、入池与转发；运行中周期复测，劣化节点先隔离再淘汰并自动补测新 IP
          </p>
        </div>
      </div>
      <span class="chip" :class="snapshot.running ? 'live' : ''">
        <i v-if="snapshot.running" class="dot" />
        {{ statusText }}
      </span>
    </header>

    <div class="row-actions">
      <button
        v-if="!snapshot.running"
        class="btn primary"
        :disabled="starting || !ipCount"
        @click="emit('start')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5l11 7-11 7V5z" /></svg>
        {{ starting ? '启动中…' : '启动代理' }}
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

    <div v-if="!scanning" class="table-wrap">
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
          <tr v-for="node in snapshot.primary" :key="`p-${node.ip}`">
            <td>
              <span v-if="node.isolated" class="tag over">隔离</span>
              <span v-else class="tag qualified">主选</span>
            </td>
            <td class="mono">{{ node.ip }}</td>
            <td class="mono dim">{{ node.colo || '—' }}</td>
            <td class="num-col mono">{{ formatLatency(node.latency) }}</td>
            <td class="num-col mono">{{ formatLoss(node.lossRate) }}</td>
            <td class="num-col mono dim">{{ node.samples }}</td>
            <td class="dim">{{ formatTime(node.updatedAt) }}</td>
          </tr>
          <tr v-for="node in snapshot.backup" :key="`b-${node.ip}`">
            <td>
              <span v-if="node.isolated" class="tag over">隔离</span>
              <span v-else class="tag excluded">备用</span>
            </td>
            <td class="mono">{{ node.ip }}</td>
            <td class="mono dim">{{ node.colo || '—' }}</td>
            <td class="num-col mono">{{ formatLatency(node.latency) }}</td>
            <td class="num-col mono">{{ formatLoss(node.lossRate) }}</td>
            <td class="num-col mono dim">{{ node.samples }}</td>
            <td class="dim">{{ formatTime(node.updatedAt) }}</td>
          </tr>

          <tr v-if="!hasNodes">
            <td colspan="7" class="empty">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 4.5a1.4 1.4 0 1 1 0 2.8 1.4 1.4 0 0 1 0-2.8zM13.2 17h-2.4v-6h2.4v6z"
                />
              </svg>
              <span v-if="snapshot.running">正在扫描候选 IP…</span>
              <span v-else-if="snapshot.phase === 'ready'">
                扫描完成但没有节点达标，请放宽延迟上限或调整 IP 来源
              </span>
              <span v-else>点击「启动代理」开始</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

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
