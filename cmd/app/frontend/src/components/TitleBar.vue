<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Window as AppWindow } from '@wailsio/runtime'

const maximised = ref(false)

async function refreshState(): Promise<void> {
  try {
    maximised.value = await AppWindow.IsMaximised()
  } catch {
    maximised.value = false
  }
}

async function toggleMaximise(): Promise<void> {
  await AppWindow.ToggleMaximise()
  await refreshState()
}

onMounted(refreshState)
</script>

<template>
  <header class="titlebar">
    <span class="titlebar-title">CFip · Cloudflare 优选 IP</span>
    <div class="win-controls">
      <button class="win-btn" aria-label="最小化" @click="AppWindow.Minimise()">
        <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M0 5h10v1H0z" /></svg>
      </button>
      <button
        class="win-btn"
        :aria-label="maximised ? '向下还原' : '最大化'"
        @click="toggleMaximise"
      >
        <svg v-if="maximised" viewBox="0 0 10 10" aria-hidden="true">
          <path d="M1.5 0h7v1H2.5v6.5H1.5V0zm2 2h6.5v7H3.5V2zm1 1v5h4.5V3H4.5z" />
        </svg>
        <svg v-else viewBox="0 0 10 10" aria-hidden="true">
          <path d="M0 0h10v10H0V0zm1 1v8h8V1H1z" />
        </svg>
      </button>
      <button class="win-btn close" aria-label="关闭" @click="AppWindow.Close()">
        <svg viewBox="0 0 10 10" aria-hidden="true">
          <path
            d="M5 4.3 8.9.4 9.5 1 5.6 4.9l3.9 3.9-.6.6L5 5.5 1.1 9.4.5 8.8l3.9-3.9L.5 1 1.1.4 5 4.3z"
          />
        </svg>
      </button>
    </div>
  </header>
</template>
