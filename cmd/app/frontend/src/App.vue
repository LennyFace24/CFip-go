<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import TitleBar from './components/TitleBar.vue'
import SideBar from './components/SideBar.vue'
import SourcePanel from './components/SourcePanel.vue'
import ProxyPanel from './components/ProxyPanel.vue'
import ResultTable from './components/ResultTable.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import ToastHost from './components/ToastHost.vue'
import { useColo } from './composables/useColo'
import { useConfig } from './composables/useConfig'
import { useLog } from './composables/useLog'
import { useProxy } from './composables/useProxy'
import { useSource } from './composables/useSource'
import { useSpeedTest } from './composables/useSpeedTest'
import type { Tab } from './types'

/* ---------------- 页面级状态 ---------------- */
const tab = ref<Tab>('speed')

/* ---------------- 业务状态（各自封装在 composable 里） ---------------- */
const {
  saved: savedConfig,
  draft,
  dirty,
  saving,
  load: loadConfig,
  save: saveConfig,
  cancel: cancelConfig,
  reset: resetConfig,
} = useConfig()

const {
  text: sourceText,
  count: ipCount,
  path: importPath,
  cidrs,
  selected,
  sample,
  loading: importing,
  cidrOnline,
  cidrError,
  refreshing,
  init: initSource,
  loadCidrs,
  importFile,
  loadBuiltin,
  clear: clearSource,
} = useSource()

const {
  rows,
  running,
  progress,
  summary,
  avgLatency,
  bestIp,
  subscribe,
  start: startSpeed,
  stop: stopSpeed,
  copyBest,
  copyOne,
} = useSpeedTest()

const { exportRows, openDir } = useLog()
const { recommended, load: loadColos } = useColo()
const {
  snapshot: pool,
  starting: poolStarting,
  subscribe: subscribePool,
  refresh: refreshPool,
  start: startPool,
  stop: stopPool,
} = useProxy()

const titles: Record<Tab, string> = {
  speed: '优选 IP 测速',
  proxy: 'IP 池',
  settings: '设置',
}

const subtitle = computed(() => {
  switch (tab.value) {
    case 'speed':
      return `当前 IP 源共 ${ipCount.value} 个待测 IP`
    case 'proxy':
      return pool.value.running
        ? `主选 ${pool.value.primary.length} / ${pool.value.primaryTarget} · 备用 ${pool.value.backup.length} / ${pool.value.backupTarget}`
        : '停止状态下池中节点保留，可随时重新启动'
    default:
      return `配置文件：${savedConfig.value.path || '（未加载）'}`
  }
})

onMounted(async () => {
  subscribe()
  subscribePool()
  await Promise.all([loadConfig(), initSource(), loadColos()])
})

</script>

<template>
  <div class="app">
    <TitleBar />

    <div class="body">
      <SideBar :tab="tab" @change="tab = $event" />

      <main class="main">
        <header class="topbar">
          <div>
            <h1>{{ titles[tab] }}</h1>
            <p class="sub">{{ subtitle }}</p>
          </div>
          <span v-if="running" class="chip live"><i class="dot" />测速中</span>
        </header>

        <Transition name="page" mode="out-in">
          <div v-if="tab === 'speed'" key="speed" class="content">
            <SourcePanel
              v-model:text="sourceText"
              v-model:selected="selected"
              v-model:sample="sample"
              :count="ipCount"
              :path="importPath"
              :cidrs="cidrs"
              :loading="importing"
              :disabled="running"
              :cidr-online="cidrOnline"
              :cidr-error="cidrError"
              :refreshing="refreshing"
              @import="importFile"
              @load="loadBuiltin"
              @clear="clearSource"
              @refresh="loadCidrs({ silent: false })"
            />

            <ResultTable
              :rows="rows"
              :running="running"
              :progress="progress"
              :summary="summary"
              :target="savedConfig.number"
              :latency-limit="savedConfig.latency"
              :avg-latency="avgLatency"
              :best-ip="bestIp"
              @start="startSpeed(sourceText, savedConfig.latency)"
              @stop="stopSpeed"
              @copy="copyBest(savedConfig.number)"
              @copy-one="copyOne"
              @export="exportRows(rows, savedConfig.latency)"
              @open-dir="openDir"
            />
          </div>

          <div v-else-if="tab === 'proxy'" key="proxy" class="content">
            <ProxyPanel
              :snapshot="pool"
              :ip-count="ipCount"
              :starting="poolStarting"
              :disabled="running"
              @start="startPool(sourceText)"
              @stop="stopPool"
              @refresh="refreshPool"
            />
          </div>

          <div v-else key="settings" class="content">
            <SettingsPanel
              v-model:draft="draft"
              :dirty="dirty"
              :saving="saving"
              :recommended="recommended"
              @save="saveConfig"
              @cancel="cancelConfig"
              @reset="resetConfig"
            />
          </div>
        </Transition>
      </main>
    </div>

    <ToastHost />
  </div>
</template>
