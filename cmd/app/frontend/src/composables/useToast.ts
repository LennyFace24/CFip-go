/**
 * 全局 Toast。
 *
 * 状态定义在模块作用域，因此它是一个单例：任何 composable 都能直接调用 notify，
 * 不需要层层传参。渲染交给 <ToastHost />。
 */

import { ref } from 'vue'
import { TOAST_DURATION_MS } from '../constants'
import type { ToastItem, ToastKind } from '../types'

const items = ref<ToastItem[]>([])
let seq = 0

function notify(text: string, kind: ToastKind = 'info', duration = TOAST_DURATION_MS): void {
  const id = ++seq
  items.value = [...items.value, { id, text, kind }]
  setTimeout(() => dismiss(id), duration)
}

function dismiss(id: number): void {
  items.value = items.value.filter((item) => item.id !== id)
}

export function useToast() {
  return { items, notify, dismiss }
}
