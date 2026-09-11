/**
 * IP 池状态。
 *
 * 池的构建与健康检查都在后端持续进行，这里只订阅 pool:update 事件、
 * 整体替换快照——不做轮询，也不在本地维护增量。
 */

import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { api, toPoolSnapshot } from '../services/api'
import type { PoolSnapshot } from '../types'
import { useToast } from './useToast'

const EVENT_UPDATE = 'pool:update'

function emptySnapshot(): PoolSnapshot {
  return {
    running: false,
    phase: 'idle',
    scanTotal: 0,
    scanDone: 0,
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

  function subscribe(): void {
    // 事件负载类型由绑定生成器提供，这里不手写标注，避免与 nil 切片等细节脱节
    Events.On(EVENT_UPDATE, (event) => {
      snapshot.value = toPoolSnapshot(event.data)
    })
  }

  async function start(text: string): Promise<void> {
    if (snapshot.value.running) return
    if (!text.trim()) {
      notify('没有可用的 IP 来源，请先在上方导入或加载内置网段', 'error')
      return
    }

    // 乐观切换：点击后立刻进入运行态，按钮马上变成「停止」，随时可中断
    snapshot.value = { ...snapshot.value, running: true, phase: 'scanning' }
    try {
      await api.proxy.startPool(text)
    } catch (e) {
      snapshot.value = { ...snapshot.value, running: false, phase: 'idle' }
      notify(`启动代理失败：${e}`, 'error')
    }
  }

  async function stop(): Promise<void> {
    try {
      await api.proxy.stopPool()
    } catch (e) {
      notify(`停止代理失败：${e}`, 'error')
    }
  }

  /** 立即复测一轮，刷新各节点的实时延迟与丢包率 */
  async function recheck(): Promise<void> {
    try {
      await api.proxy.recheck()
    } catch (e) {
      notify(`复测失败：${e}`, 'error')
    }
  }

  return { snapshot, subscribe, start, stop, recheck }
}
