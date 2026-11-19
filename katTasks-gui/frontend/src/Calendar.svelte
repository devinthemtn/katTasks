<script lang="ts">
  // The calendar: a month of due dates, and the chosen day's tasks. All
  // state is in Go; clicks are sent back as picks.
  import type { Calendar } from '../bindings/github.com/devinthemtn/katTasks/internal/ui/models';
  import TaskLine from './TaskLine.svelte';

  let { cal, onpick, ontask, onmonth }: {
    cal: Calendar;
    onpick: (i: number, open: boolean) => void; // a day; open lists it
    ontask: (i: number, open: boolean) => void; // a task of the chosen day; open edits it
    onmonth: (dir: number) => void;
  } = $props();

  const cls = (st: string) => (st ? 'c-' + st.replace('_', '-') : '');
  const sel = $derived(cal.days?.[cal.selected]);
</script>

<div class="cal">
  <div class="line calhead">
    <button class="prompt" onclick={() => onmonth(-1)} aria-label="previous month">‹</button>
    <span class="prompt">{cal.title}</span>
    <button class="prompt" onclick={() => onmonth(1)} aria-label="next month">›</button>
    <span class="dim calcount">{cal.due === 1 ? '1 task' : `${cal.due} tasks`} due</span>
  </div>
  <div class="calgrid">
    {#each cal.weekdays ?? [] as w}<div class="dim bold calwd">{w}</div>{/each}
    {#each cal.days ?? [] as d, i (d.date)}
      <div class="calday" class:out={!d.inMonth} class:sel={i === cal.selected} class:today={d.today}
        onclick={(e) => onpick(i, e.detail >= 2)} role="button" tabindex="-1" title={d.date}>
        <div class="calnum">
          <span class:c-today={d.today} class:bold={d.today}>{d.day}</span>
          {#if d.open > 0}<span class="calmark {d.past ? 'c-overdue' : 'c-due'}">•{d.open}</span>
          {:else if d.total > 0}<span class="calmark dim">✓</span>{/if}
        </div>
        {#each d.items ?? [] as it}
          <div class="calitem {it.done ? 'c-done done' : cls(it.priStyle) || 'c-fg'}">{it.text}</div>
        {/each}
      </div>
    {/each}
  </div>
  <div class="calday-tasks">
    <div class="line"><span class="dim bold">{cal.dayTitle}</span><span class="dim">{' · '}{(cal.tasks?.length ?? 0) === 1 ? '1 task' : `${cal.tasks?.length ?? 0} tasks`}</span></div>
    {#each cal.tasks ?? [] as r (r.index)}
      <TaskLine {r} onclick={(e) => ontask(r.index, e.detail >= 2)} />
    {/each}
    {#if !cal.tasks?.length && sel}
      <div class="line dim">&nbsp;&nbsp;nothing due — a adds a task for this day</div>
    {/if}
  </div>
</div>
