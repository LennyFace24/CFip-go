/**
 * IP 池状态。
 *
 * 池的构建与健康检查都在后端持续进行，这里只订阅 pool:update 事件、
 * 整体替换快照——不做轮询，也不在本地维护增量。
 */

import { computed, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { api, toPoolSnapshot } from '../services/api'
import type { PoolSnapshot } from '../types'
import { useToast } from './useToast'

const EVENT_UPDATE = 'pool:update'

function emptySnapshot(): PoolSnapshot {
  return {
    running: false,
    primaryTarget: 0,
    backupTarget: 0,
    listenAddr: '',
    listenError: '',
    activeConns: 0,
    primary: [],
    backup: [],
    evictions: [],
  }
}

export function useProxy() {
  const { notify } = useToast()

  const snapshot = ref<PoolSnapshot>(emptySnapshot())
  const starting = ref(false)

  const total = computed(() => snapshot.value.primary.length + snapshot.value.backup.length)

  function subscribe(): void {
    // 事件负载类型由绑定生成器提供，这里不手写标注，避免与 nil 切片等细节脱节
    Events.On(EVENT_UPDATE, (event) => {
      snapshot.value = toPoolSnapshot(event.data)
    })
  }

  /** 首次进入页面时拉一次，避免错过订阅前的事件 */
  async function refresh(): Promise<void> {
    try {
      snapshot.value = await api.proxy.snapshot()
    } catch (e) {
      notify(`读取 IP 池失败：${e}`, 'error')
    }
  }

  async function start(text: string): Promise<void> {
    if (snapshot.value.running) return
    if (!text.trim()) {
      notify('没有可用的 IP 来源，请先在测速页导入或加载内置网段', 'error')
      return
    }
    starting.value = true
    try {
      await api.proxy.startPool(text)
    } catch (e) {
      notify(`启动 IP 池失败：${e}`, 'error')
    } finally {
      starting.value = false
    }
  }

  async function stop(): Promise<void> {
    try {
      await api.proxy.stopPool()
    } catch (e) {
      notify(`停止 IP 池失败：${e}`, 'error')
    }
  }

  return { snapshot, starting, total, subscribe, refresh, start, stop }
}
