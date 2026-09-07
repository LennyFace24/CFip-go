<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Config } from '../types'

const props = defineProps<{
  draft: Config
  dirty: boolean
  saving: boolean
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

const fields: {
  key: keyof Omit<Config, 'path'>
  label: string
  hint: string
  unit: string
}[] = [
  { key: 'latency', label: '延迟上限', hint: '探测成功但延迟超过它算「超标」，不计入达标数量', unit: 'ms' },
  { key: 'concurrency', label: '并发数', hint: '同时发起探测的协程数量，越大越快但占用更多资源', unit: '个' },
  { key: 'timeout', label: '请求超时', hint: '单个 IP 探测的超时时间', unit: 'ms' },
  { key: 'number', label: '达标 IP 数量', hint: '集满该数量的达标 IP 后自动停止测速', unit: '个' },
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
          <h2>测速参数</h2>
          <p class="card-sub">修改后需点击保存才会写入配置文件</p>
        </div>
      </div>
      <span v-if="props.dirty" class="chip warn">未保存</span>
    </header>

    <div class="setting-list">
      <div v-for="field in fields" :key="field.key" class="setting-item">
        <div class="setting-text">
          <span class="setting-title">{{ field.label }}</span>
          <span class="setting-desc">{{ field.hint }}</span>
        </div>
        <div class="setting-control">
          <input
            v-model.number="local[field.key]"
            class="num"
            type="number"
            min="1"
            @change="commit"
          />
          <em>{{ field.unit }}</em>
        </div>
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
