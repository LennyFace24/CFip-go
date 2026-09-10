<script setup lang="ts">
import type { Tab } from '../types'

defineProps<{ tab: Tab }>()
const emit = defineEmits<{ change: [tab: Tab] }>()

interface NavItem {
  key: Tab
  label: string
  icon: string
}

const items: NavItem[] = [
  { key: 'speed', label: '测速', icon: 'M13 2 3 14h8l-1 8 10-12h-8l1-8z' },
  {
    key: 'proxy',
    label: 'IP 池',
    icon:
      'M18 16.1a3 3 0 0 0-2.1.9l-6.1-3.6a3.1 3.1 0 0 0 0-2.8l6.1-3.6a3 3 0 1 0-1-2.1c0 .3 0 .5.1.8L8.9 9.3a3 3 0 1 0 0 5.4l6.1 3.6c0 .2-.1.5-.1.7a3 3 0 1 0 3-3z',
  },
  {
    key: 'settings',
    label: '设置',
    icon:
      'M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zm7.4-2.5a7.7 7.7 0 0 0 0-2l2-1.5-2-3.5-2.3 1a7.6 7.6 0 0 0-1.7-1L15 3H9l-.4 2a7.6 7.6 0 0 0-1.7 1l-2.3-1-2 3.5L4.6 11a7.7 7.7 0 0 0 0 2l-2 1.5 2 3.5 2.3-1c.5.4 1.1.8 1.7 1L9 21h6l.4-2c.6-.2 1.2-.6 1.7-1l2.3 1 2-3.5-2-1.5z',
  },
]
</script>

<template>
  <aside class="sidebar">
    
    <nav class="nav">
      <button
        v-for="item in items"
        :key="item.key"
        class="nav-item"
        :class="{ active: tab === item.key }"
        @click="emit('change', item.key)"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path :d="item.icon" /></svg>
        <span>{{ item.label }}</span>
      </button>
    </nav>

    <footer class="side-foot">Wails v3 · Go</footer>
  </aside>
</template>
