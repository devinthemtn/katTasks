<script lang="ts">
  // A line of styled text pieces from the task list (Seg).
  import type { Seg } from '../bindings/github.com/devinthemtn/katTasks/internal/ui/models';

  let { segs }: { segs: Seg[] } = $props();

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

{#each segs as g}<span class={cls(g.style)}>{g.text}</span>{/each}
