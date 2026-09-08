<script setup lang="ts">
import { useRouter } from 'vue-router';
import { session } from './api';

const router = useRouter();

async function logout() {
  await session.logout();
  router.push('/login');
}
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <strong>木十的博客 · 后台</strong>
      <nav v-if="session.user">
        <router-link to="/media">媒体库</router-link>
        <router-link to="/build">构建</router-link>
        <router-link to="/stats">统计</router-link>
      </nav>
      <span class="spacer"></span>
      <span v-if="session.user" class="muted">{{ session.user }} · {{ session.info?.storage }}</span>
      <button v-if="session.user" type="button" class="ghost" @click="logout">退出</button>
    </header>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>
