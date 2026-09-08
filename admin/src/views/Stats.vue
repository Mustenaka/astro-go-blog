<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api } from '../api';

interface Counter {
  slug: string;
  views: number;
  likes: number;
}
const topViews = ref<Counter[]>([]);
const topLikes = ref<Counter[]>([]);
const totals = ref({ views: 0, likes: 0 });
const error = ref('');

onMounted(async () => {
  try {
    const res = await api.get<{ topViews: Counter[]; topLikes: Counter[]; totalViews: number; totalLikes: number }>('/stats');
    topViews.value = res.topViews;
    topLikes.value = res.topLikes;
    totals.value = { views: res.totalViews, likes: res.totalLikes };
  } catch (e) {
    error.value = (e as Error).message;
  }
});
</script>

<template>
  <h1>统计</h1>
  <p class="muted">总浏览 {{ totals.views }} · 总点赞 {{ totals.likes }}</p>
  <p v-if="error" class="error">{{ error }}</p>
  <div class="row" style="align-items: flex-start">
    <section class="panel" style="flex: 1; min-width: 20rem">
      <h2>浏览前二十</h2>
      <table>
        <thead><tr><th>#</th><th>slug</th><th>浏览</th><th>点赞</th></tr></thead>
        <tbody>
          <tr v-for="(c, i) in topViews" :key="c.slug"><td>{{ i + 1 }}</td><td><code>{{ c.slug }}</code></td><td>{{ c.views }}</td><td>{{ c.likes }}</td></tr>
          <tr v-if="!topViews.length"><td colspan="4" class="muted">暂无数据</td></tr>
        </tbody>
      </table>
    </section>
    <section class="panel" style="flex: 1; min-width: 20rem">
      <h2>点赞前二十</h2>
      <table>
        <thead><tr><th>#</th><th>slug</th><th>点赞</th><th>浏览</th></tr></thead>
        <tbody>
          <tr v-for="(c, i) in topLikes" :key="c.slug"><td>{{ i + 1 }}</td><td><code>{{ c.slug }}</code></td><td>{{ c.likes }}</td><td>{{ c.views }}</td></tr>
          <tr v-if="!topLikes.length"><td colspan="4" class="muted">暂无数据</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>
