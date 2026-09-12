<script setup lang="ts">
import { ref, watch } from 'vue'
import { toggleColo, splitColo } from '../composables/useColo'
import type { ColoOption, Config } from '../types'

const props = defineProps<{
  draft: Config
  dirty: boolean
  saving: boolean
  /** 内置推荐机房，用于快捷选择 */
  recommended: ColoOption[]
}>()

const emit = defineEmits<{
  'update:draft': [value: Config]
  save: []
  cancel: []
  reset: []
}>()

// props 不可直接改，用本地副本编辑，改动后整体 emit 给父组件
const local = ref<Config>({ ...props.draft })
watch(
  () => props.draft,
  (value) => {
    local.value = { ...value }
  },
)

function commit(): void {
  emit('update:draft', { ...local.value })
}

function onColo(event: Event): void {
  local.value.colo = (event.target as HTMLInputElement).value
  commit()
}

function onProxyListen(event: Event): void {
  local.value.proxyListen = (event.target as HTMLInputElement).value
  commit()
}

function isColoOn(code: string): boolean {
  return splitColo(local.value.colo).includes(code.toUpperCase())
}

function switchColo(code: string): void {
  local.value.colo = toggleColo(local.value.colo, code)
  commit()
}

interface FieldDef {
  key: keyof Omit<Config, 'path' | 'colo'>
  label: string
  hint: string
  unit: string
  min: number
  step: number
}

const sections: { title: string; fields: FieldDef[] }[] = [
  {
    title: '测速',
    fields: [
      { key: 'latency', label: '延迟上限', hint: '探测成功但延迟超过它算「超标」，不计入达标数量', unit: 'ms', min: 1, step: 10 },
      { key: 'concurrency', label: '并发数', hint: '同时发起探测的协程数量，越大越快但占用更多资源', unit: '个', min: 1, step: 1 },
      { key: 'timeout', label: '请求超时', hint: '单个 IP 探测的超时时间', unit: 'ms', min: 1, step: 100 },
      { key: 'number', label: '达标 IP 数量', hint: '集满该数量的达标 IP 后自动停止测速', unit: '个', min: 1, step: 1 },
    ],
  },
  {
    title: 'IP 池',
    fields: [
      { key: 'primarySize', label: '主选节点数', hint: '承载实际流量的节点数量', unit: '个', min: 1, step: 1 },
      { key: 'backupSize', label: '备用节点数', hint: '主选出现空缺时按延迟递补', unit: '个', min: 0, step: 1 },
      { key: 'cooldown', label: '冷却时长', hint: '节点被淘汰后多久内不参与补位，用于防抖动', unit: '秒', min: 0, step: 30 },
    ],
  },
  {
    title: '健康检查',
    fields: [
      { key: 'healthInterval', label: '检查周期', hint: '多久对池中节点复测一轮', unit: '秒', min: 1, step: 10 },
      { key: 'pingTimes', label: '每次采样次数', hint: '单个节点一轮内探测几次，用于算丢包率', unit: '次', min: 1, step: 1 },
      { key: 'pingGap', label: '采样间隔', hint: '同一次检查内相邻两次探测的间隔', unit: 'ms', min: 0, step: 50 },
      { key: 'lossLimit', label: '丢包率上限', hint: '需与采样次数配套：采样 5 次时 0.25 表示允许其中 1 次失败', unit: '', min: 0, step: 0.05 },
    ],
  },
]
</script>

<template>
  <section class="card">
    <header class="card-head">
      <div class="card-title">
        <svg class="card-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zm7.4-2.5a7.7 7.7 0 0 0 0-2l2-1.5-2-3.5-2.3 1a7.6 7.6 0 0 0-1.7-1L15 3H9l-.4 2a7.6 7.6 0 0 0-1.7 1l-2.3-1-2 3.5L4.6 11a7.7 7.7 0 0 0 0 2l-2 1.5 2 3.5 2.3-1c.5.4 1.1.8 1.7 1L9 21h6l.4-2c.6-.2 1.2-.6 1.7-1l2.3 1 2-3.5-2-1.5z"
          />
        </svg>
        <div>
          <h2>参数设置</h2>
          <p class="card-sub">修改后需点击保存才会写入配置文件</p>
        </div>
      </div>
      <span v-if="props.dirty" class="chip warn">未保存</span>
    </header>

    <div v-for="section in sections" :key="section.title" class="setting-group">
      <h3 class="group-title">{{ section.title }}</h3>
      <div class="setting-list">
        <div v-for="field in section.fields" :key="field.key" class="setting-item">
          <div class="setting-text">
            <span class="setting-title">{{ field.label }}</span>
            <span class="setting-desc">{{ field.hint }}</span>
          </div>
          <div class="setting-control">
            <input
              v-model.number="local[field.key]"
              class="num"
              type="number"
              :min="field.min"
              :step="field.step"
              @change="commit"
            />
            <em v-if="field.unit">{{ field.unit }}</em>
          </div>
        </div>
      </div>
    </div>

    <div class="setting-group">
      <h3 class="group-title">代理</h3>
      <div class="setting-item">
        <div class="setting-text">
          <span class="setting-title">SOCKS5 监听地址</span>
          <span class="setting-desc">本地代理监听地址，形如 127.0.0.1:1234</span>
        </div>
        <div class="setting-control">
          <input
            class="input listen-input"
            type="text"
            :value="local.proxyListen"
            placeholder="127.0.0.1:1234"
            @change="onProxyListen"
          />
        </div>
      </div>
    </div>

    <div class="colo-block">
      <div class="setting-text">
        <span class="setting-title">机房白名单</span>
        <span class="setting-desc">
          空格分隔的 IATA 代码，如 HKG NRT SIN；留空则不过滤。白名单过窄可能一个都选不出来
        </span>
      </div>
      <input
        class="input colo-input"
        type="text"
        :value="local.colo"
        placeholder="留空 = 不过滤"
        @change="onColo"
      />
      <div class="chips">
        <button
          v-for="colo in props.recommended"
          :key="colo.code"
          type="button"
          class="chip-toggle"
          :class="{ on: isColoOn(colo.code) }"
          @click="switchColo(colo.code)"
        >
          {{ colo.code }} · {{ colo.name }}
        </button>
      </div>
    </div>

    <div class="row-actions end">
      <button class="btn ghost" :disabled="props.saving" @click="emit('reset')">恢复默认</button>
      <button class="btn ghost" :disabled="!props.dirty || props.saving" @click="emit('cancel')">
        取消
      </button>
      <button class="btn primary" :disabled="!props.dirty || props.saving" @click="emit('save')">
        {{ props.saving ? '保存中…' : '保存' }}
      </button>
    </div>

    <footer class="card-foot mono">配置文件：{{ props.draft.path || '（未加载）' }}</footer>
  </section>
</template>
