<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { api, type BuildJob } from '../api';

const jobs = ref<BuildJob[]>([]);
const current = ref<BuildJob | null>(null);
const log = ref('');
const error = ref('');
const busy = ref(false);
let timer: number | undefined;

async function refreshList() {
  const res = await api.get<{ jobs: BuildJob[] }>('/build');
  jobs.value = res.jobs;
  if (!current.value && res.jobs[0]) await select(res.jobs[0].id);
}

async function select(id: number) {
  const res = await api.get<{ job: BuildJob; log: string }>(`/build/${id}`);
  current.value = res.job;
  log.value = res.log;
}

async function trigger() {
  error.value = '';
  busy.value = true;
  try {
    const res = await api.post<{ job: BuildJob }>('/build');
    current.value = res.job;
    log.value = '';
    await refreshList();
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}

async function poll() {
  try {
    if (current.value && (current.value.status === 'running' || current.value.status === 'queued')) {
      await select(current.value.id);
      await refreshList();
    }
  } catch (e) {
    error.value = (e as Error).message;
  }
}

onMounted(async () => {
  await refreshList().catch((e) => (error.value = e.message));
  timer = window.setInterval(poll, 1500);
});
onBeforeUnmount(() => window.clearInterval(timer));
</script>

<template>
  <h1>构建</h1>
  <section class="panel">
    <div class="row">
      <button class="primary" type="button" :disabled="busy" data-testid="trigger-build" @click="trigger">触发构建</button>
      <span v-if="current" class="muted">
        #{{ current.id }} <span class="badge" :class="current.status" data-testid="build-status">{{ current.status }}</span>
        {{ current.trigger }} · {{ current.startedAt ?? current.createdAt }}
      </span>
      <p v-if="error" class="error" style="margin: 0">{{ error }}</p>
    </div>
    <pre class="log" data-testid="build-log" style="margin-top: 1rem">{{ log || '（暂无日志）' }}</pre>
  </section>
  <section class="panel">
    <h2>最近十次</h2>
    <table>
      <thead><tr><th>#</th><th>状态</th><th>来源</th><th>开始</th><th>结束</th><th>退出码</th></tr></thead>
      <tbody>
        <tr v-for="j in jobs" :key="j.id" style="cursor: pointer" @click="select(j.id)">
          <td>{{ j.id }}</td>
          <td><span class="badge" :class="j.status">{{ j.status }}</span></td>
          <td>{{ j.trigger }}</td>
          <td class="muted">{{ j.startedAt ?? '' }}</td>
          <td class="muted">{{ j.finishedAt ?? '' }}</td>
          <td>{{ j.exitCode ?? '' }}</td>
        </tr>
        <tr v-if="!jobs.length"><td colspan="6" class="muted">还没有构建记录</td></tr>
      </tbody>
    </table>
  </section>
</template>
