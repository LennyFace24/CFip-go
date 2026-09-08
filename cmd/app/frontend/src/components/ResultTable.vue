<script setup lang="ts">
import { computed, ref } from 'vue'
import StatTile from './StatTile.vue'
import { SORT_MODES } from '../constants'
import type { Progress, Row, RowState, SortMode } from '../types'

const props = defineProps<{
  rows: Row[]
  running: boolean
  progress: Progress
  summary: string
  /** 目标：需要凑够的达标数量 */
  target: number
  /** 延迟上限（毫秒），超过即视为超标 */
  latencyLimit: number
  avgLatency: string
  bestIp: string
}>()

const emit = defineEmits<{
  start: []
  stop: []
  copy: []
  copyOne: [ip: string]
  export: []
  openDir: []
}>()

const keyword = ref('')
const sortMode = ref<SortMode>('latency')

function stateOf(row: Row): RowState {
  if (!row.allowed) return 'excluded'
  if (row.latency < 0) return 'failed'
  return row.latency <= props.latencyLimit ? 'qualified' : 'over'
}

const stateLabel: Record<RowState, string> = {
  qualified: '达标',
  over: '超标',
  failed: '失败',
  excluded: '机房不符',
}

const filtered = computed<Row[]>(() => {
  const k = keyword.value.trim().toLowerCase()
  const list = k ? props.rows.filter((row) => row.ip.toLowerCase().includes(k)) : [...props.rows]

  switch (sortMode.value) {
    case 'ip':
      return list.sort((a, b) => a.ip.localeCompare(b.ip))
    case 'status':
      return list.sort((a, b) => Number(b.latency >= 0) - Number(a.latency >= 0))
    default:
      return list.sort((a, b) => {
        const okA = a.latency >= 0
        const okB = b.latency >= 0
        if (okA !== okB) return okA ? -1 : 1
        if (!okA) return 0
        return a.latency - b.latency
      })
  }
})

const percent = computed(() =>
  props.target > 0 ? Math.min(100, Math.round((props.progress.qualified / props.target) * 100)) : 0,
)
</script>

<template>
  <section class="card">
    <header class="card-head">
      <div class="card-title">
        <svg class="card-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M3 3v18h18M7 15.5 11 11l3 2.5L20 7"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
        <div>
          <h2>测速结果</h2>
          <p class="card-sub">
            已探测 {{ props.progress.done }} · 达标 {{ props.progress.qualified }} / {{ props.target }}
            · 上限 {{ props.latencyLimit }} ms
          </p>
        </div>
      </div>

      <div class="row-actions">
        <button v-if="!props.running" class="btn primary" @click="emit('start')">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5l11 7-11 7V5z" /></svg>
          开始测速
        </button>
        <button v-else class="btn danger" @click="emit('stop')">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 7h10v10H7z" /></svg>
          停止
        </button>
        <button class="btn ghost" :disabled="!props.rows.length" @click="emit('copy')">
          复制优选
        </button>
        <button class="btn ghost" :disabled="!props.rows.length" @click="emit('export')">
          导出日志
        </button>
        <button class="btn ghost" @click="emit('openDir')">日志目录</button>
      </div>
    </header>

    <div class="progress">
      <div class="progress-bar" :class="{ indeterminate: props.running }">
        <span :style="{ width: percent + '%' }" />
      </div>
      <span class="progress-text">{{ props.progress.qualified }} / {{ props.target }}</span>
    </div>

    <div class="stats">
      <StatTile label="达标" :value="props.progress.qualified" tone="ok" />
      <StatTile label="超标" :value="props.progress.over" tone="warn" />
      <StatTile label="失败" :value="props.progress.failed" tone="bad" />
      <StatTile label="机房不符" :value="props.progress.excluded" tone="muted" />
      <StatTile label="平均延迟 (ms)" :value="props.avgLatency" />
      <StatTile label="最快 IP" :value="props.bestIp" mono />
    </div>

    <div class="table-tools">
      <input v-model="keyword" class="input" type="text" placeholder="按 IP 筛选…" />
      <div class="sorts">
        <button
          v-for="mode in SORT_MODES"
          :key="mode.key"
          class="sort"
          :class="{ on: sortMode === mode.key }"
          @click="sortMode = mode.key"
        >
          {{ mode.label }}
        </button>
      </div>
    </div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>IP</th>
            <th class="num-col">延迟 (ms)</th>
            <th>机房</th>
            <th>状态</th>
            <th>来源</th>
            <th class="act-col" />
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in filtered" :key="row.seq">
            <td class="mono">{{ row.ip }}</td>
            <td class="num-col mono">{{ row.latency >= 0 ? row.latency.toFixed(1) : '—' }}</td>
            <td class="mono dim">{{ row.colo || '—' }}</td>
            <td>
              <span class="tag" :class="stateOf(row)">{{ stateLabel[stateOf(row)] }}</span>
            </td>
            <td class="dim">{{ row.source }}</td>
            <td class="act-col">
              <button
                v-if="row.latency >= 0"
                class="icon-btn"
                :title="`复制 ${row.ip}`"
                @click="emit('copyOne', row.ip)"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path
                    d="M9 2h9v14H9zM6 6v16h9v-2H8V6H6z"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    stroke-linejoin="round"
                  />
                </svg>
              </button>
            </td>
          </tr>

          <tr v-if="!filtered.length">
            <td colspan="6" class="empty">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 4.5a1.4 1.4 0 1 1 0 2.8 1.4 1.4 0 0 1 0-2.8zM13.2 17h-2.4v-6h2.4v6z"
                />
              </svg>
              <span>{{ props.running ? '正在测速，结果将陆续出现…' : '暂无结果，先设置 IP 来源再开始测速' }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <footer v-if="props.summary" class="card-foot">{{ props.summary }}</footer>
  </section>
</template>
