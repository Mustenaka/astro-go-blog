<script setup lang="ts">
import { ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ApiError, session } from '../api';

const username = ref('admin');
const password = ref('');
const error = ref('');
const busy = ref(false);
const router = useRouter();
const route = useRoute();

async function submit() {
  error.value = '';
  busy.value = true;
  try {
    await session.login(username.value, password.value);
    router.push(typeof route.query.next === 'string' ? route.query.next : '/media');
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <form class="panel login" @submit.prevent="submit">
    <h1>登录</h1>
    <label>用户名 <input v-model="username" autocomplete="username" required /></label>
    <label>密码 <input v-model="password" type="password" autocomplete="current-password" required /></label>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <button class="primary" type="submit" :disabled="busy">{{ busy ? '登录中…' : '登录' }}</button>
  </form>
</template>
