<script setup lang="ts">
/**
 * Artalk comment island (shell). Loads the Artalk client from the configured server and
 * mounts it. The Astro side only renders this component when PUBLIC_ARTALK_SERVER is set.
 */
import { onBeforeUnmount, onMounted, ref } from 'vue';

const props = defineProps<{ server: string; site: string; pageKey: string; pageTitle: string }>();

const el = ref<HTMLElement | null>(null);
const status = ref<'loading' | 'ready' | 'error'>('loading');
let instance: { destroy?: () => void } | null = null;

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) return resolve();
    const s = document.createElement('script');
    s.src = src;
    s.async = true;
    s.onload = () => resolve();
    s.onerror = () => reject(new Error(`failed to load ${src}`));
    document.head.appendChild(s);
  });
}

onMounted(async () => {
  const base = props.server.replace(/\/+$/, '');
  try {
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = `${base}/dist/Artalk.css`;
    document.head.appendChild(link);
    await loadScript(`${base}/dist/Artalk.js`);
    const Artalk = (window as unknown as { Artalk?: { init: (cfg: Record<string, unknown>) => { destroy?: () => void } } }).Artalk;
    if (!Artalk || !el.value) throw new Error('Artalk global missing');
    instance = Artalk.init({ el: el.value, server: base, site: props.site, pageKey: props.pageKey, pageTitle: props.pageTitle });
    status.value = 'ready';
  } catch {
    status.value = 'error';
  }
});

onBeforeUnmount(() => {
  instance?.destroy?.();
});
</script>

<template>
  <section class="comments" aria-label="评论">
    <div ref="el"></div>
    <p v-if="status === 'loading'" class="comments-placeholder">评论加载中……</p>
    <p v-else-if="status === 'error'" class="comments-placeholder">评论服务暂不可用。</p>
  </section>
</template>
