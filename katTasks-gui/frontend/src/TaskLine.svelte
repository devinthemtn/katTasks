<script lang="ts">
  // One task row, drawn as the terminal UI's renderTask draws it.
  import type { TaskRow } from '../bindings/github.com/devinthemtn/katTasks/internal/ui/models';

  let { r, numW = 0, cur = false, marked = false, onclick, ontoggle }: {
    r: TaskRow;
    numW?: number;
    cur?: boolean; // the cursor is on it (and its pane is focused)
    marked?: boolean;
    onclick?: (e: MouseEvent) => void;
    ontoggle?: (e: MouseEvent) => void;
  } = $props();

  const bold = (p: string) => (p === 'pri_a' || p === 'pri_b' || p === 'pri_c' ? 'bold' : '');
  const cls = (st: string) => 'c-' + st.replace('_', '-');
</script>

<div class="line task" class:cur class:marked={marked && !cur} {onclick} role="button" tabindex="-1">
  <span class="prompt">{r.cursor ? '▌' : r.marked ? '•' : ' '}</span>{#if numW > 0}<span class="dim pre">{String(r.num).padStart(numW)}&nbsp;</span>{/if}<span
    class="box {r.done ? 'c-ok' : 'dim'}" onclick={ontoggle} role="checkbox" aria-checked={r.done} tabindex="-1">{r.done ? '[x]' : '[ ]'}&nbsp;</span><span
    class="pri pre">{#if r.priority}<span class="{cls(r.priStyle)} {bold(r.priStyle)}">({r.priority})&nbsp;</span>{:else}&nbsp;&nbsp;&nbsp;&nbsp;{/if}</span><span
    class="text">{#each r.words ?? [] as w, i}{#if i > 0}&nbsp;{/if}<span class={cls(w.style)} class:done={w.style === 'done'} class:match={w.match}>{w.text}</span>{/each}</span><span
    class="badge">{#if r.recur}<span class="c-context">&nbsp;↻</span>{/if}{#if r.due}<span class={cls(r.dueStyle)} class:bold={r.dueStyle === 'today' || r.dueStyle === 'overdue'}>&nbsp;{r.due}&nbsp;</span>{:else if r.recur}&nbsp;{/if}</span>
</div>
