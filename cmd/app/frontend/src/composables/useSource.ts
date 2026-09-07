/**
 * IP 来源状态：文本真源、内置网段选择、导入文件。
 *
 * 约定：文本域是唯一真源，导入与加载内置网段都只是往里追加内容。
 * 内置网段由后端决定来源：优先在线拉取，失败时回退编译期内置列表。
 */

import { computed, ref, watch } from 'vue'
import { api } from '../services/api'
import { DEFAULT_SAMPLE, PARSE_DEBOUNCE_MS } from '../constants'
import { useToast } from './useToast'

export function useSource() {
  const { notify } = useToast()

  const text = ref('')
  const count = ref(0)
  const path = ref('')
  const cidrs = ref<string[]>([])
  const selected = ref<string[]>([])
  const sample = ref(DEFAULT_SAMPLE)
  const loading = ref(false)

  /** 网段列表是否为在线拉取所得 */
  const cidrOnline = ref(false)
  /** 在线拉取失败的原因，空串表示无失败 */
  const cidrError = ref('')
  /** 是否正在（重新）拉取网段 */
  const refreshing = ref(false)

  const allSelected = computed(
    () => cidrs.value.length > 0 && selected.value.length === cidrs.value.length,
  )

  // 文本变化后防抖重新统计，避免大文件每敲一个字就解析一次
  let timer: ReturnType<typeof setTimeout> | undefined
  watch(text, () => {
    clearTimeout(timer)
    timer = setTimeout(refreshCount, PARSE_DEBOUNCE_MS)
  })

  async function refreshCount(): Promise<void> {
    try {
      count.value = await api.ip.parse(text.value)
    } catch {
      count.value = 0
    }
  }

  /** 拉取网段列表；silent 为 true 时只在失败且用户主动刷新时提示 */
  async function loadCidrs(options: { silent?: boolean } = {}): Promise<void> {
    const { silent = true } = options
    refreshing.value = true
    try {
      const source = await api.ip.builtinCidrs()
      cidrs.value = source.cidrs
      cidrOnline.value = source.online
      cidrError.value = source.error
      // 只保留仍然存在于新列表中的旧选择
      const valid = new Set(source.cidrs)
      const kept = selected.value.filter((cidr) => valid.has(cidr))
      selected.value = kept.length ? kept : [...source.cidrs]

      if (!silent) {
        notify(source.online ? '已更新为官方最新网段' : `拉取失败，已回退内置网段：${source.error}`, source.online ? 'success' : 'error')
      }
    } catch (e) {
      cidrs.value = []
      cidrOnline.value = false
      cidrError.value = String(e)
      if (!silent) notify(`拉取网段失败：${e}`, 'error')
    } finally {
      refreshing.value = false
    }
  }

  async function init(): Promise<void> {
    await Promise.all([loadCidrs(), refreshCount()])
  }

  function append(addition: string): void {
    const next = addition.trim()
    if (!next) return
    const current = text.value.trim()
    text.value = current ? `${current}\n${next}` : next
  }

  async function importFile(): Promise<void> {
    loading.value = true
    try {
      const result = await api.ip.importFile()
      append(result.content)
      path.value = result.path
      notify(`已导入 ${result.count} 个 IP`, 'success')
    } catch (e) {
      notify(`导入失败：${e}`, 'error')
    } finally {
      loading.value = false
    }
  }

  async function loadBuiltin(): Promise<void> {
    if (!selected.value.length) {
      notify('请至少选择一个网段', 'error')
      return
    }
    try {
      const content = await api.ip.builtinText(selected.value, sample.value)
      append(content)
      notify(`已加载 ${selected.value.length} 个内置网段`, 'success')
    } catch (e) {
      notify(`加载内置网段失败：${e}`, 'error')
    }
  }

  function clear(): void {
    text.value = ''
    path.value = ''
    count.value = 0
  }

  function toggleAll(): void {
    selected.value = allSelected.value ? [] : [...cidrs.value]
  }

  return {
    text,
    count,
    path,
    cidrs,
    selected,
    sample,
    loading,
    cidrOnline,
    cidrError,
    refreshing,
    allSelected,
    init,
    loadCidrs,
    importFile,
    loadBuiltin,
    clear,
    toggleAll,
  }
}
