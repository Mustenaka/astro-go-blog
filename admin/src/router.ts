import { createRouter, createWebHashHistory } from 'vue-router';
import { session } from './api';
import Login from './views/Login.vue';
import Media from './views/Media.vue';
import Build from './views/Build.vue';
import Stats from './views/Stats.vue';

export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/media' },
    { path: '/login', component: Login, meta: { public: true } },
    { path: '/media', component: Media },
    { path: '/build', component: Build },
    { path: '/stats', component: Stats },
  ],
});

router.beforeEach(async (to) => {
  if (to.meta.public) return true;
  if (!(await session.ensure())) return { path: '/login', query: { next: to.fullPath } };
  return true;
});
