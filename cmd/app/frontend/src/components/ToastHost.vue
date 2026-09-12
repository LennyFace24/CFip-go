<script setup lang="ts">
import { useToast } from '../composables/useToast'

const { items, dismiss } = useToast()
</script>

<template>
  <div class="toast-host">
    <TransitionGroup name="toast">
      <div
        v-for="item in items"
        :key="item.id"
        class="toast"
        :class="item.kind"
        role="status"
        @click="dismiss(item.id)"
      >
        {{ item.text }}
      </div>
    </TransitionGroup>
  </div>
</template>
