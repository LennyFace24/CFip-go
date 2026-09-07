<script setup lang="ts">
import { ref } from 'vue'
import CidrPicker from './CidrPicker.vue'

const props = defineProps<{
  text: string
  count: number
  path: string
  cidrs: string[]
  selected: string[]
  sample: number
  loading: boolean
  disabled: boolean
  cidrOnline: boolean
  cidrError: string
  refreshing: boolean
}>()

const emit = defineEmits<{
  'update:text': [value: string]
  'update:selected': [value: string[]]
  'update:sample': [value: number]
  import: []
  load: []
  clear: []
  refresh: []
}>()

const expanded = ref(false)

function onInput(event: Event): void {
  emit('update:text', (event.target as HTMLTextAreaElement).value)
}
</script>

<template>
  <section class="card">
    <header class="card-head">
      <div class="card-title">
        <svg class="card-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M4 6h16v12H4z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linejoin="round"
          />
          <circle cx="8.4" cy="12" r="1.15" />
          <circle cx="15.6" cy="12" r="1.15" />
        </svg>
        <div>
          <h2>IP 来源</h2>
          <p class="card-sub">导入 ip.txt、加载内置 Cloudflare 网段，或直接编辑</p>
        </div>
      </div>
      <span class="chip">{{ props.count }} 个待测 IP</span>
    </header>

    <div class="row-actions">
      <button class="btn" :disabled="props.disabled || props.loading" @click="emit('import')">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M12 3v11m0 0 4-4m-4 4-4-4M4 19h16"
            fill="none"
            stroke="currentColor"
            stroke-width="1.9"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
        {{ props.loading ? '导入中…' : '导入 ip.txt' }}
      </button>

      <button class="btn" :class="{ active: expanded }" @click="expanded = !expanded">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M4 6h16M4 12h16M4 18h10"
            fill="none"
            stroke="currentColor"
            stroke-width="1.9"
            stroke-linecap="round"
          />
        </svg>
        内置网段
      </button>

      <button class="btn ghost" :disabled="props.disabled || !props.text" @click="emit('clear')">
        清空
      </button>
    </div>

    <CidrPicker
      v-if="expanded"
      :cidrs="props.cidrs"
      :selected="props.selected"
      :sample="props.sample"
      :disabled="props.disabled"
      :online="props.cidrOnline"
      :error="props.cidrError"
      :refreshing="props.refreshing"
      @update:selected="emit('update:selected', $event)"
      @update:sample="emit('update:sample', $event)"
      @load="emit('load')"
      @refresh="emit('refresh')"
    />

    <textarea
      class="editor"
      :value="props.text"
      :disabled="props.disabled"
      spellcheck="false"
      placeholder="每行一条，支持：&#10;1.1.1.1&#10;1.1.1.1/13&#10;1.1.1.1/13=500"
      @input="onInput"
    />

    <footer v-if="props.path" class="card-foot">已导入：{{ props.path }}</footer>
  </section>
</template>
