<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import TitleBar from './components/TitleBar.vue'
import SideBar from './components/SideBar.vue'
import SourcePanel from './components/SourcePanel.vue'
import ProxyPanel from './components/ProxyPanel.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import ToastHost from './components/ToastHost.vue'
import { useColo } from './composables/useColo'
import { useConfig } from './composables/useConfig'
import { useLog } from './composables/useLog'
import { useProxy } from './composables/useProxy'
import { useSource } from './composables/useSource'
import type { Tab } from './types'

/* ---------------- 页面级状态 ---------------- */
const tab = ref<Tab>('proxy')

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
  snapshot: pool,
  subscribe: subscribePool,
  start: startPool,
  stop: stopPool,
  recheck: recheckPool,
} = useProxy()

const { exportPool, openDir } = useLog()
const { recommended, load: loadColos } = useColo()

const titles: Record<Tab, string> = { proxy: '代理', settings: '设置' }

const subtitle = computed(() => {
  if (tab.value === 'settings') {
    return `配置文件：${savedConfig.value.path || '（未加载）'}`
  }

  const current = pool.value
  if (current.phase === 'scanning') {
    return `正在扫描候选 IP　${current.scanDone} / ${current.scanTotal}`
  }
  if (current.running) {
    const listen = current.listenAddr ? ` · ${current.listenAddr}` : ''
    return `主选 ${current.primary.length} / ${current.primaryTarget} · 备用 ${current.backup.length} / ${current.backupTarget}${listen}`
  }
  return '一键完成测速、优选、入池与转发'
})

onMounted(async () => {
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
          <span v-if="pool.running" class="chip live">
            {{ pool.phase === 'scanning' ? '扫描中' : '运行中' }}
          </span>
        </header>

        <Transition name="page" mode="out-in">
          <div v-if="tab === 'proxy'" key="proxy" class="content">
            <SourcePanel
              v-model:text="sourceText"
              v-model:selected="selected"
              v-model:sample="sample"
              :count="ipCount"
              :path="importPath"
              :cidrs="cidrs"
              :loading="importing"
              :disabled="pool.running"
              :cidr-online="cidrOnline"
              :cidr-error="cidrError"
              :refreshing="refreshing"
              @import="importFile"
              @load="loadBuiltin"
              @clear="clearSource"
              @refresh="loadCidrs({ silent: false })"
            />

            <ProxyPanel
              :snapshot="pool"
              :ip-count="ipCount"
              @start="startPool(sourceText)"
              @stop="stopPool"
              @recheck="recheckPool"
              @export="exportPool(pool)"
              @open-dir="openDir"
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
