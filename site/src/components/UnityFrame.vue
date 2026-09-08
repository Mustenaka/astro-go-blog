<script setup lang="ts">
/**
 * Lazy iframe for Unity WebGL builds (or any hosted HTML demo). Nothing loads until the user
 * clicks; the click target shows the download size so people on metered connections can decide.
 */
import { ref } from 'vue';

const props = defineProps<{ src: string; aspect: string; sizeHint?: string; title?: string }>();

const started = ref(false);
const frame = ref<HTMLIFrameElement | null>(null);
const wrapper = ref<HTMLElement | null>(null);

function start() {
  started.value = true;
}

async function fullscreen() {
  const el = wrapper.value;
  if (!el) return;
  if (document.fullscreenElement) await document.exitFullscreen();
  else await el.requestFullscreen?.();
}
</script>

<template>
  <div ref="wrapper" class="unity-embed" :style="{ aspectRatio: props.aspect }">
    <button v-if="!started" type="button" class="unity-start" data-testid="unity-start" @click="start">
      <span class="unity-start-label">▶ 加载并运行</span>
      <span v-if="props.sizeHint" class="unity-start-size">需要下载约 {{ props.sizeHint }}</span>
    </button>
    <template v-else>
      <iframe
        ref="frame"
        :src="props.src"
        :title="props.title ?? 'Unity WebGL'"
        allow="fullscreen; autoplay; xr-spatial-tracking"
        allowfullscreen
        loading="lazy"
        data-testid="unity-frame"
      ></iframe>
      <button type="button" class="unity-fullscreen" aria-label="全屏" title="全屏" @click="fullscreen">⛶</button>
    </template>
  </div>
</template>

<style scoped>
.unity-embed {
  position: relative;
  width: 100%;
  background: #111;
  border-radius: var(--radius-md, 0.5rem);
  overflow: hidden;
}
.unity-embed iframe {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  border: 0;
}
.unity-start {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  width: 100%;
  border: 0;
  background: linear-gradient(135deg, #1f2937, #111);
  color: #f9fafb;
  cursor: pointer;
  font: inherit;
}
.unity-start-label {
  font-size: 1.1rem;
  font-weight: 600;
}
.unity-start-size {
  font-size: 0.85rem;
  opacity: 0.75;
}
.unity-fullscreen {
  position: absolute;
  right: 0.5rem;
  bottom: 0.5rem;
  border: 1px solid rgb(255 255 255 / 0.3);
  background: rgb(0 0 0 / 0.5);
  color: #fff;
  border-radius: 0.25rem;
  padding: 0.15rem 0.5rem;
  cursor: pointer;
}
</style>
