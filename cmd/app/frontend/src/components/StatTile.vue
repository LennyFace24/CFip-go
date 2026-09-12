<script setup lang="ts">
/** 统计磁贴：数值 + 说明，用于结果概览 */
withDefaults(
  defineProps<{
    label: string
    value: string | number
    tone?: 'default' | 'ok' | 'warn' | 'bad' | 'muted'
    mono?: boolean
  }>(),
  { tone: 'default', mono: false },
)
</script>

<template>
  <div class="stat">
    <span class="stat-value" :class="[tone, { mono }]">{{ value }}</span>
    <span class="stat-label">{{ label }}</span>
  </div>
</template>
