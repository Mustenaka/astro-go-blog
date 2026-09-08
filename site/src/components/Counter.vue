<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{ start?: number }>();
const count = ref(props.start ?? 0);
</script>

<template>
  <div class="counter">
    <button type="button" @click="count--">-</button>
    <span data-testid="count">{{ count }}</span>
    <button type="button" @click="count++">+</button>
  </div>
</template>

<style scoped>
.counter {
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 1rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
}
button {
  min-width: 2rem;
  padding: 0.25rem 0.5rem;
  border: 1px solid #9ca3af;
  border-radius: 0.25rem;
  background: #f3f4f6;
  cursor: pointer;
}
</style>
