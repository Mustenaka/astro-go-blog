<script setup lang="ts">
/**
 * View counter + like button for an article. Talks to the Go API (PUBLIC_API_BASE; empty means
 * same origin). Silent when the API is unreachable so the static page never breaks.
 */
import { onMounted, ref } from 'vue';

const props = defineProps<{ slug: string; apiBase: string }>();

const views = ref<number | null>(null);
const likes = ref<number | null>(null);
const liked = ref(false);
const busy = ref(false);
const unavailable = ref(false);
const base = props.apiBase.replace(/\/+$/, '');

async function call(method: 'GET' | 'POST', path: string) {
  const res = await fetch(`${base}${path}`, { method, credentials: 'omit' });
  if (!res.ok) throw new Error(String(res.status));
  return res.json();
}

onMounted(async () => {
  try {
    const data = await call('POST', `/api/v1/counters/${props.slug}/view`);
    views.value = data.views;
    likes.value = data.likes;
  } catch {
    unavailable.value = true;
  }
});

async function like() {
  if (busy.value || unavailable.value) return;
  busy.value = true;
  try {
    const data = await call('POST', `/api/v1/likes/${props.slug}`);
    likes.value = data.likes;
    liked.value = true;
  } catch {
    unavailable.value = true;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="counters" v-if="!unavailable">
    <span class="counters-views" data-testid="views">
      <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7-11-7-11-7z" /><circle cx="12" cy="12" r="3" />
      </svg>
      {{ views ?? '–' }} 次浏览
    </span>
    <button type="button" class="counters-like" :class="{ liked }" :disabled="busy" data-testid="like" @click="like" aria-label="点赞">
      <svg viewBox="0 0 24 24" width="16" height="16" :fill="liked ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
      </svg>
      <span data-testid="likes">{{ likes ?? '–' }}</span>
    </button>
  </div>
</template>

<style scoped>
.counters {
  display: inline-flex;
  align-items: center;
  gap: 1rem;
  font-size: var(--text-sm, 0.9rem);
  color: var(--color-text-muted, #666);
}
.counters-views {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}
.counters-like {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border: 1px solid var(--color-border, #ddd);
  background: transparent;
  color: inherit;
  border-radius: var(--radius-md, 0.5rem);
  padding: 0.25rem 0.75rem;
  cursor: pointer;
}
.counters-like:hover,
.counters-like.liked {
  color: var(--color-accent, #0f766e);
  border-color: var(--color-accent, #0f766e);
}
</style>
