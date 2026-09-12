/**
 * 机房（colo）白名单。
 *
 * 白名单以「空格分隔的 IATA 代码」形式存在配置里（如 "HKG NRT SIN"），
 * 留空表示不过滤。这里只负责推荐列表与纯文本的增删，判定交给后端。
 */

import { ref } from 'vue'
import { api } from '../services/api'
import type { ColoOption } from '../types'

/** 把白名单文本拆成代码数组，空输入返回空数组 */
export function splitColo(codes: string): string[] {
  return codes
    .split(/[\s,，、;；]+/)
    .map((code) => code.trim().toUpperCase())
    .filter(Boolean)
}

/** 在白名单文本中切换某个代码：已有则移除，没有则追加 */
export function toggleColo(current: string, code: string): string {
  const list = splitColo(current)
  const target = code.trim().toUpperCase()
  const index = list.indexOf(target)
  if (index >= 0) {
    list.splice(index, 1)
  } else {
    list.push(target)
  }
  return list.join(' ')
}

export function useColo() {
  const recommended = ref<ColoOption[]>([])

  async function load(): Promise<void> {
    try {
      recommended.value = await api.ip.recommendedColos()
    } catch {
      recommended.value = []
    }
  }

  return { recommended, load, toggleColo, splitColo }
}
