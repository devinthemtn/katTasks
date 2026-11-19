<script lang="ts">
  // A line of styled text pieces from the task list (Seg). Pieces that name
  // an action run it when clicked.
  import type { Seg } from '../bindings/github.com/devinthemtn/katTasks/internal/ui/models';

  let { segs, onclick }: { segs: Seg[]; onclick?: (action: string) => void } = $props();

  function cls(style: string | undefined) {
    switch (style) {
      case 'key': return 'bold';
      case 'chip': return 'chip';
      case 'prompt': return 'prompt';
      case 'ok': return 'c-ok';
      case 'error': return 'c-error bold';
      case undefined: case '': return '';
    }
    return 'c-' + style.replace('_', '-');
  }
</script>

{#each segs as g}{#if g.action && onclick}<span class="{cls(g.style)} action" onclick={() => onclick(g.action!)} role="button" tabindex="-1">{g.text}</span>{:else}<span class={cls(g.style)}>{g.text}</span>{/if}{/each}
