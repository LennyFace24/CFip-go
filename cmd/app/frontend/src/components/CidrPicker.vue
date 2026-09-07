<script setup lang="ts">
import { computed } from 'vue'
import { MAX_SAMPLE, MIN_SAMPLE } from '../constants'

const props = defineProps<{
  cidrs: string[]
  selected: string[]
  sample: number
  disabled: boolean
  online: boolean
  error: string
  refreshing: boolean
}>()

const emit = defineEmits<{
  'update:selected': [value: string[]]
  'update:sample': [value: number]
  load: []
  refresh: []
}>()

const allSelected = computed(
  () => props.cidrs.length > 0 && props.selected.length === props.cidrs.length,
)

function isOn(cidr: string): boolean {
  return props.selected.includes(cidr)
}

function toggle(cidr: string): void {
  const next = isOn(cidr)
    ? props.selected.filter((item) => item !== cidr)
    : [...props.selected, cidr]
  emit('update:selected', next)
}

function toggleAll(): void {
  emit('update:selected', allSelected.value ? [] : [...props.cidrs])
}

function onSample(event: Event): void {
  emit('update:sample', Number((event.target as HTMLInputElement).value))
}
</script>

<template>
  <div class="picker">
    <div class="picker-bar">
      <button class="link" :disabled="disabled" @click="toggleAll">
        {{ allSelected ? '全部取消' : '全部选择' }}
      </button>
      <span class="picker-count">{{ props.selected.length }} / {{ props.cidrs.length }}</span>

      <label class="picker-sample">
        每网段采样
        <input
          class="num small"
          type="number"
          :value="props.sample"
          :min="MIN_SAMPLE"
          :max="MAX_SAMPLE"
          :disabled="disabled"
          @change="onSample"
        />
      </label>

      <button class="btn primary sm" :disabled="disabled || !props.selected.length" @click="emit('load')">
        加载所选
      </button>
    </div>

    <p class="source-note" :class="{ warn: !props.online }">
      <span v-if="props.online">来源：Cloudflare 官方（在线更新）</span>
      <span v-else>来源：内置列表（在线拉取失败{{ props.error ? `：${props.error}` : '' }}）</span>
      <button class="link" :disabled="props.refreshing" @click="emit('refresh')">
        {{ props.refreshing ? '更新中…' : '重新拉取' }}
      </button>
    </p>

    <div class="chips">
      <button
        v-for="cidr in props.cidrs"
        :key="cidr"
        type="button"
        class="chip-toggle"
        :class="{ on: isOn(cidr) }"
        :disabled="disabled"
        @click="toggle(cidr)"
      >
        {{ cidr }}
      </button>
    </div>
  </div>
</template>
