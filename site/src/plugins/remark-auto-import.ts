/**
 * Auto-imports the shared MDX components. Content files use `<Demo />`, `<Video />`,
 * `<UnityEmbed />` and `<Counter />` without writing `import` lines: this plugin adds the
 * import for every registered component that appears in the file and is not imported already.
 *
 * Why: Keystatic's MDX editor cannot represent ESM import statements, and AI-written content
 * should not need to know component paths. The mapping lives in one place: `autoImports` below.
 */

export const autoImports: Record<string, string> = {
  Demo: '@components/Demo.astro',
  Video: '@components/Video.astro',
  UnityEmbed: '@components/UnityEmbed.astro',
  Counter: '@components/Counter.astro',
};

interface MdastNode {
  type: string;
  name?: string | null;
  value?: string;
  children?: MdastNode[];
  data?: Record<string, unknown>;
}

function importNode(name: string, source: string): MdastNode {
  const value = `import ${name} from ${JSON.stringify(source)};`;
  return {
    type: 'mdxjsEsm',
    value,
    data: {
      estree: {
        type: 'Program',
        sourceType: 'module',
        body: [
          {
            type: 'ImportDeclaration',
            specifiers: [{ type: 'ImportDefaultSpecifier', local: { type: 'Identifier', name } }],
            source: { type: 'Literal', value: source, raw: JSON.stringify(source) },
          },
        ],
      },
    },
  };
}

// Typed loosely on purpose: mdast types are not resolvable from this package and the plugin only
// touches the few node shapes declared above.
export function remarkAutoImport(): (tree: any) => void {
  return (tree: MdastNode) => {
    const used = new Set<string>();
    const imported = new Set<string>();

    const visit = (node: MdastNode) => {
      if ((node.type === 'mdxJsxFlowElement' || node.type === 'mdxJsxTextElement') && node.name) {
        const root = node.name.split('.')[0]!;
        if (root in autoImports) used.add(root);
      } else if (node.type === 'mdxjsEsm' && node.value) {
        for (const m of node.value.matchAll(/import\s+(?:\{[^}]*\}|([A-Za-z_$][\w$]*))/g)) {
          if (m[1]) imported.add(m[1]);
          if (m[0].includes('{')) for (const n of m[0].matchAll(/([A-Za-z_$][\w$]*)\s*(?:,|\})/g)) imported.add(n[1]!);
        }
      }
      node.children?.forEach(visit);
    };
    visit(tree);

    const missing = [...used].filter((name) => !imported.has(name));
    if (missing.length === 0 || !tree.children) return;
    tree.children.unshift(...missing.map((name) => importNode(name, autoImports[name]!)));
  };
}
