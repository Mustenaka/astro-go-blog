/**
 * Demo registry: name -> async Vue component loader. Each demo lives in `src/demos/<name>/index.vue`
 * and is loaded only when the `<Demo name="..." />` island for it becomes visible, so heavy
 * libraries (three.js) end up in their own chunk and never load on pages without that demo.
 *
 * To add a demo: create the folder, add one line here. `<Demo>` fails the build for unknown names.
 */
import type { Component } from 'vue';

export const demos = {
  particles: () => import('./particles/index.vue'),
} satisfies Record<string, () => Promise<{ default: Component }>>;

export type DemoName = keyof typeof demos;

export const demoNames = Object.keys(demos) as DemoName[];

export function isDemoName(name: string): name is DemoName {
  return Object.prototype.hasOwnProperty.call(demos, name);
}
