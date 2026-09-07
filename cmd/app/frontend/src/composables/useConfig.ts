/**
 * 配置状态。
 *
 * 保存「已保存值」与「草稿值」两份：设置面板只改草稿，
 * 保存成功后再同步回已保存值，取消就是丢弃草稿。
 */

import { computed, ref } from 'vue'
import { api } from '../services/api'
import { DEFAULT_CONFIG } from '../constants'
import type { Config } from '../types'
import { useToast } from './useToast'

function emptyConfig(): Config {
  return { ...DEFAULT_CONFIG, path: '' }
}

export function useConfig() {
  const { notify } = useToast()

  const saved = ref<Config>(emptyConfig())
  const draft = ref<Config>(emptyConfig())
  const saving = ref(false)

  const dirty = computed(
    () => JSON.stringify(saved.value) !== JSON.stringify(draft.value),
  )

  async function load(): Promise<void> {
    try {
      saved.value = await api.config.get()
      draft.value = { ...saved.value }
    } catch (e) {
      notify(`读取配置失败：${e}`, 'error')
    }
  }

  async function save(): Promise<void> {
    saving.value = true
    try {
      await api.config.save(draft.value)
      saved.value = { ...draft.value }
      notify('配置已保存', 'success')
    } catch (e) {
      notify(`保存失败：${e}`, 'error')
    } finally {
      saving.value = false
    }
  }

  function cancel(): void {
    draft.value = { ...saved.value }
  }

  async function reset(): Promise<void> {
    try {
      const defaults = await api.config.defaults()
      draft.value = { ...defaults, path: saved.value.path }
      notify('已填入默认配置，记得保存')
    } catch (e) {
      notify(`读取默认配置失败：${e}`, 'error')
    }
  }

  function updateDraft(next: Config): void {
    draft.value = next
  }

  return { saved, draft, saving, dirty, load, save, cancel, reset, updateDraft }
}
