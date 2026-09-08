<script setup lang="ts">
/**
 * Particle demo: a few hundred points falling under gravity inside a box, bouncing off the
 * walls, with a mild swirl. Two sliders (count, gravity) show that demos are interactive.
 *
 * Resource rules: everything created in onMounted is released in onBeforeUnmount (geometry,
 * material, renderer, listeners, RAF). The render loop pauses while the canvas is off-screen.
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import * as THREE from 'three';

const host = ref<HTMLDivElement | null>(null);
const count = ref(600);
const gravity = ref(9.8);
const running = ref(false);

let renderer: THREE.WebGLRenderer | null = null;
let scene: THREE.Scene | null = null;
let camera: THREE.PerspectiveCamera | null = null;
let points: THREE.Points | null = null;
let geometry: THREE.BufferGeometry | null = null;
let material: THREE.PointsMaterial | null = null;
let positions = new Float32Array(0);
let velocities = new Float32Array(0);
let raf = 0;
let last = 0;
let observer: IntersectionObserver | null = null;
let resizeObserver: ResizeObserver | null = null;

const BOX = 1.5;

function rebuild() {
  if (!scene) return;
  if (points) {
    scene.remove(points);
    geometry?.dispose();
  }
  const n = count.value;
  positions = new Float32Array(n * 3);
  velocities = new Float32Array(n * 3);
  for (let i = 0; i < n; i++) {
    positions[i * 3] = (Math.random() - 0.5) * BOX * 2;
    positions[i * 3 + 1] = Math.random() * BOX * 2 - BOX;
    positions[i * 3 + 2] = (Math.random() - 0.5) * BOX * 2;
    velocities[i * 3] = (Math.random() - 0.5) * 2;
    velocities[i * 3 + 1] = 0;
    velocities[i * 3 + 2] = (Math.random() - 0.5) * 2;
  }
  geometry = new THREE.BufferGeometry();
  geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
  material ??= new THREE.PointsMaterial({ color: 0x5eead4, size: 0.05, sizeAttenuation: true });
  points = new THREE.Points(geometry, material);
  scene.add(points);
}

function step(dt: number) {
  const g = gravity.value;
  const n = positions.length / 3;
  for (let i = 0; i < n; i++) {
    const ix = i * 3, iy = ix + 1, iz = ix + 2;
    // mild swirl around the vertical axis + gravity
    const x = positions[ix]!, z = positions[iz]!;
    velocities[ix]! += -z * 0.6 * dt;
    velocities[iz]! += x * 0.6 * dt;
    velocities[iy]! -= g * dt;
    positions[ix]! += velocities[ix]! * dt;
    positions[iy]! += velocities[iy]! * dt;
    positions[iz]! += velocities[iz]! * dt;
    for (const [p, v] of [[ix, ix], [iy, iy], [iz, iz]] as const) {
      if (positions[p]! < -BOX) {
        positions[p] = -BOX;
        velocities[v] = -velocities[v]! * 0.75;
      } else if (positions[p]! > BOX) {
        positions[p] = BOX;
        velocities[v] = -velocities[v]! * 0.75;
      }
    }
  }
  const attr = geometry?.getAttribute('position') as THREE.BufferAttribute | undefined;
  if (attr) attr.needsUpdate = true;
}

function loop(now: number) {
  if (!running.value || !renderer || !scene || !camera) return;
  const dt = Math.min(0.033, (now - last) / 1000 || 0.016);
  last = now;
  step(dt);
  if (points) points.rotation.y += dt * 0.15;
  renderer.render(scene, camera);
  raf = requestAnimationFrame(loop);
}

function start() {
  if (running.value) return;
  running.value = true;
  last = performance.now();
  raf = requestAnimationFrame(loop);
}

function stop() {
  running.value = false;
  cancelAnimationFrame(raf);
}

function resize() {
  if (!host.value || !renderer || !camera) return;
  const w = host.value.clientWidth;
  const h = Math.round(w * 0.6);
  renderer.setSize(w, h, false);
  camera.aspect = w / h;
  camera.updateProjectionMatrix();
}

onMounted(() => {
  if (!host.value) return;
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: 'low-power' });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  host.value.appendChild(renderer.domElement);
  renderer.domElement.style.width = '100%';
  renderer.domElement.style.display = 'block';
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  camera.position.set(0, 0.6, 4.5);
  camera.lookAt(0, 0, 0);
  const box = new THREE.LineSegments(new THREE.EdgesGeometry(new THREE.BoxGeometry(BOX * 2, BOX * 2, BOX * 2)), new THREE.LineBasicMaterial({ color: 0x9aa4ae }));
  scene.add(box);
  rebuild();
  resize();
  resizeObserver = new ResizeObserver(resize);
  resizeObserver.observe(host.value);
  observer = new IntersectionObserver((entries) => {
    for (const e of entries) {
      if (e.isIntersecting) start();
      else stop();
    }
  });
  observer.observe(host.value);
});

onBeforeUnmount(() => {
  stop();
  observer?.disconnect();
  resizeObserver?.disconnect();
  scene?.traverse((obj) => {
    const mesh = obj as THREE.Mesh;
    mesh.geometry?.dispose?.();
    const mat = mesh.material as THREE.Material | THREE.Material[] | undefined;
    if (Array.isArray(mat)) mat.forEach((m) => m.dispose());
    else mat?.dispose?.();
  });
  geometry?.dispose();
  material?.dispose();
  renderer?.dispose();
  renderer?.domElement.remove();
  renderer = scene = camera = points = geometry = material = null;
});

watch(count, rebuild);
</script>

<template>
  <div class="particles-demo">
    <div ref="host" class="particles-canvas" data-testid="particles-canvas"></div>
    <div class="particles-controls">
      <label>
        粒子数 <output>{{ count }}</output>
        <input v-model.number="count" type="range" min="100" max="2000" step="100" data-testid="count-slider" />
      </label>
      <label>
        重力 <output>{{ gravity.toFixed(1) }}</output>
        <input v-model.number="gravity" type="range" min="0" max="20" step="0.5" data-testid="gravity-slider" />
      </label>
      <span class="particles-state" data-testid="running">{{ running ? '运行中' : '已暂停' }}</span>
    </div>
  </div>
</template>

<style scoped>
.particles-demo {
  border: 1px solid var(--color-border, #ddd);
  border-radius: var(--radius-md, 0.5rem);
  overflow: hidden;
  background: var(--color-surface, #f6f7f8);
}
.particles-canvas {
  width: 100%;
  min-height: 200px;
}
.particles-controls {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem 1.5rem;
  align-items: center;
  padding: 0.6rem 0.9rem;
  border-top: 1px solid var(--color-border, #ddd);
  font-size: var(--text-sm, 0.9rem);
  color: var(--color-text-muted, #666);
}
.particles-controls label {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}
.particles-controls output {
  min-width: 2.5em;
  font-variant-numeric: tabular-nums;
}
.particles-state {
  margin-left: auto;
  font-size: var(--text-xs, 0.8rem);
}
</style>
