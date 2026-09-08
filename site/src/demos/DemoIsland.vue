<script setup lang="ts">
/**
 * The single island that hosts every demo. It resolves the component through the registry at
 * runtime (dynamic import = separate chunk) and shows a placeholder until it arrives.
 */
import { defineAsyncComponent, h, type Component } from 'vue';
import { demos, isDemoName } from './registry';

const props = defineProps<{ name: string }>();

const Loading: Component = {
  render: () => h('div', { class: 'demo-loading' }, '加载 demo…'),
};

const Failed: Component = {
  render: () => h('div', { class: 'demo-error' }, `demo「${props.name}」加载失败`),
};

const DemoComponent = isDemoName(props.name)
  ? defineAsyncComponent({ loader: demos[props.name], loadingComponent: Loading, errorComponent: Failed, delay: 0 })
  : Failed;
</script>

<template>
  <div class="demo-island" :data-demo="name">
    <DemoComponent />
  </div>
</template>
