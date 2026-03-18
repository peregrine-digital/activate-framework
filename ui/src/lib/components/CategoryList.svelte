<script lang="ts">
  import type { FileStatus, Category } from '../types.js';
  import FileCard from './FileCard.svelte';

  interface CategoryGroup {
    category: string;
    label: string;
    files: FileStatus[];
  }

  interface Props {
    files: FileStatus[];
    categories: Category[];
    installed: boolean;
    sectionPrefix: string;
    skippedVersions: Record<string, string>;
    onInstall: (file: FileStatus) => void;
    onUninstall: (file: FileStatus) => void;
    onDiff: (file: FileStatus) => void;
    onSkipUpdate: (file: FileStatus) => void;
    onOpen: (file: FileStatus) => void;
    onSetOverride: (dest: string, override: '' | 'pinned' | 'excluded') => void;
  }

  const CATEGORY_ICONS: Record<string, string> = {
    instructions: '📝',
    prompts: '💬',
    skills: '🛠',
    agents: '🤖',
    'mcp-servers': '🔌',
    other: '📄',
  };

  let { files, categories, installed, sectionPrefix, skippedVersions, onInstall, onUninstall, onDiff, onSkipUpdate, onOpen, onSetOverride }: Props = $props();

  let groups = $derived.by(() => {
    const grouped: Record<string, FileStatus[]> = {};
    for (const f of files) {
      const cat = f.category || 'other';
      if (!grouped[cat]) grouped[cat] = [];
      grouped[cat].push(f);
    }

    const order = categories.length > 0 ? categories.map((c) => c.id) : Object.keys(grouped);
    const labelMap: Record<string, string> = {};
    for (const c of categories) labelMap[c.id] = c.label;

    const result: CategoryGroup[] = [];
    for (const cat of order) {
      if (grouped[cat]) {
        result.push({ category: cat, label: labelMap[cat] || cat, files: grouped[cat] });
      }
    }
    for (const cat of Object.keys(grouped)) {
      if (!result.some((g) => g.category === cat)) {
        result.push({ category: cat, label: cat, files: grouped[cat] });
      }
    }
    return result;
  });
</script>

{#each groups as group}
  <details class="mb-0.5">
    <summary class="cursor-pointer py-1.5 px-1 font-semibold text-xs rounded select-none hover:bg-activate-bg-hover
      [&::-webkit-details-marker]:hidden list-none
      before:content-['▸\_'] before:text-[10px] before:inline
      [details[open]>&]:before:content-['▾\_']">
      {CATEGORY_ICONS[group.category] || '📄'} {group.label}
      <span class="opacity-50 font-normal ml-1">{group.files.length}</span>
    </summary>
    {#each group.files as file}
      <FileCard
        {file}
        {installed}
        skippedVersion={skippedVersions[file.dest]}
        {onInstall}
        {onUninstall}
        {onDiff}
        {onSkipUpdate}
        {onOpen}
        {onSetOverride}
      />
    {/each}
  </details>
{/each}
