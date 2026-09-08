<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { api, formatBytes, session, uploadFile, type Asset } from '../api';

const prefixes = computed(() => session.info?.prefixes ?? ['img', 'video', 'demos', 'models', 'files']);
const prefix = ref('img');
const demoName = ref('');
const demoVersion = ref('');
const filterPrefix = ref('');
const query = ref('');
const page = ref(1);
const total = ref(0);
const items = ref<Asset[]>([]);
const error = ref('');
const dragging = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

interface Upload {
  name: string;
  progress: number;
  status: 'uploading' | 'done' | 'error';
  message?: string;
  asset?: Asset;
}
const uploads = ref<Upload[]>([]);

async function load() {
  error.value = '';
  try {
    const params = new URLSearchParams({ page: String(page.value) });
    if (filterPrefix.value) params.set('prefix', filterPrefix.value);
    if (query.value.trim()) params.set('q', query.value.trim());
    const res = await api.get<{ items: Asset[]; total: number; page: number }>(`/assets?${params}`);
    items.value = res.items;
    total.value = res.total;
  } catch (e) {
    error.value = String((e as Error).message);
  }
}
onMounted(load);
watch([filterPrefix, page], load);
let searchTimer: number | undefined;
watch(query, () => {
  window.clearTimeout(searchTimer);
  searchTimer = window.setTimeout(() => {
    page.value = 1;
    load();
  }, 250);
});

async function handleFiles(files: FileList | File[]) {
  for (const file of Array.from(files)) {
    const entry: Upload = { name: file.name, progress: 0, status: 'uploading' };
    uploads.value.unshift(entry);
    try {
      const demo =
        prefix.value === 'demos'
          ? { name: demoName.value.trim(), version: demoVersion.value.trim(), relativePath: (file as File & { webkitRelativePath?: string }).webkitRelativePath?.split('/').slice(1).join('/') || file.name }
          : undefined;
      entry.asset = await uploadFile(file, prefix.value, (p) => (entry.progress = p), demo);
      entry.status = 'done';
    } catch (e) {
      entry.status = 'error';
      entry.message = (e as Error).message;
    }
  }
  await load();
}

function onDrop(e: DragEvent) {
  dragging.value = false;
  if (e.dataTransfer?.files?.length) handleFiles(e.dataTransfer.files);
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    window.prompt('复制路径', text);
  }
}

async function remove(asset: Asset) {
  if (!window.confirm(`删除 ${asset.key}？此操作不可恢复。`)) return;
  try {
    await api.del(`/assets/${asset.id}`);
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

const isImage = (a: Asset) => a.contentType.startsWith('image/');
const pages = computed(() => Math.max(1, Math.ceil(total.value / 20)));
</script>

<template>
  <h1>媒体库</h1>

  <section class="panel">
    <h2>上传</h2>
    <div class="row" style="margin-bottom: 0.75rem">
      <label>
        前缀
        <select v-model="prefix">
          <option v-for="p in prefixes" :key="p" :value="p">{{ p }}/</option>
        </select>
      </label>
      <template v-if="prefix === 'demos'">
        <label>demo 名 <input v-model="demoName" placeholder="cube" /></label>
        <label>版本 <input v-model="demoVersion" placeholder="1.0.0" /></label>
      </template>
      <span class="muted" v-else>路径按 {{ prefix }}/YYYY/MM/ 自动生成</span>
    </div>
    <div
      class="dropzone"
      :class="{ active: dragging }"
      @dragover.prevent="dragging = true"
      @dragleave="dragging = false"
      @drop.prevent="onDrop"
      @click="fileInput?.click()"
    >
      拖拽文件到这里，或点击选择
      <input ref="fileInput" type="file" multiple hidden data-testid="file-input" @change="fileInput?.files && handleFiles(fileInput.files)" />
    </div>
    <ul v-if="uploads.length" style="list-style: none; padding: 0; margin: 1rem 0 0">
      <li v-for="u in uploads" :key="u.name + u.progress" style="margin-bottom: 0.6rem">
        <div class="row">
          <span>{{ u.name }}</span>
          <span v-if="u.status === 'done'" class="ok">完成</span>
          <span v-else-if="u.status === 'error'" class="error">{{ u.message }}</span>
          <span v-else class="muted">{{ Math.round(u.progress * 100) }}%</span>
          <template v-if="u.asset">
            <code data-testid="uploaded-path">{{ u.asset.assetPath }}</code>
            <button type="button" @click="copy(u.asset!.assetPath)">复制路径</button>
          </template>
        </div>
        <div class="progress" v-if="u.status === 'uploading'"><div :style="{ width: u.progress * 100 + '%' }"></div></div>
      </li>
    </ul>
  </section>

  <section class="panel">
    <div class="row" style="margin-bottom: 0.75rem">
      <h2 style="margin: 0">资产</h2>
      <select v-model="filterPrefix">
        <option value="">全部前缀</option>
        <option v-for="p in prefixes" :key="p" :value="p">{{ p }}/</option>
      </select>
      <input v-model="query" placeholder="搜索文件名或路径" />
      <span class="spacer"></span>
      <span class="muted">{{ total }} 项</span>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <table>
      <thead>
        <tr><th></th><th>路径</th><th>原文件名</th><th>大小</th><th>类型</th><th>时间</th><th></th></tr>
      </thead>
      <tbody>
        <tr v-for="a in items" :key="a.id">
          <td><img v-if="isImage(a)" class="thumb" :src="a.publicUrl" alt="" loading="lazy" /></td>
          <td><code>{{ a.assetPath }}</code></td>
          <td>{{ a.originalName }}</td>
          <td>{{ formatBytes(a.size) }}</td>
          <td class="muted">{{ a.contentType }}</td>
          <td class="muted">{{ a.createdAt.slice(0, 16).replace('T', ' ') }}</td>
          <td class="row">
            <button type="button" @click="copy(a.assetPath)">复制</button>
            <button type="button" class="danger ghost" @click="remove(a)">删除</button>
          </td>
        </tr>
        <tr v-if="!items.length"><td colspan="7" class="muted">没有资产</td></tr>
      </tbody>
    </table>
    <div class="row" style="margin-top: 0.75rem" v-if="pages > 1">
      <button type="button" :disabled="page <= 1" @click="page--">上一页</button>
      <span class="muted">{{ page }} / {{ pages }}</span>
      <button type="button" :disabled="page >= pages" @click="page++">下一页</button>
    </div>
  </section>
</template>
