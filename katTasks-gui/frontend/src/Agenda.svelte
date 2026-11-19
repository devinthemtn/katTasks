<script lang="ts">
  // The agenda: a week of columns. Each day's untimed tasks sit at the top;
  // timed ones are placed down the day by time on an hour grid. The tasks
  // are the list's own, so clicks select and edit them as in the list.
  import type { Agenda, AgendaItem, TaskRow } from '../bindings/github.com/devinthemtn/katTasks/internal/ui/models';

  let { ag, focused, onclick, onweek }: {
    ag: Agenda;
    focused: boolean;
    onclick: (r: TaskRow, e: MouseEvent) => void;
    onweek: (dir: number) => void;
  } = $props();

  const HOUR = 2.5; // lines per hour
  const ITEM = 1.15; // lines per timed task

  const hours = $derived(Array.from({ length: Math.max(1, ag.toHour - ag.fromHour) }, (_, i) => ag.fromHour + i));
  const height = $derived(hours.length * HOUR);

  // Timed tasks at their time, pushed down where they'd overlap.
  function place(items: AgendaItem[] | null) {
    let bottom = -Infinity;
    return (items ?? []).map((it) => {
      const top = Math.max((it.minutes / 60 - ag.fromHour) * HOUR, bottom);
      bottom = top + ITEM;
      return { it, top };
    });
  }

  const cls = (st: string) => (st ? 'c-' + st.replace('_', '-') : '');

  // The current time, for a line across today.
  let now = $state(new Date());
  $effect(() => {
    const id = setInterval(() => (now = new Date()), 60_000);
    return () => clearInterval(id);
  });
  const nowTop = $derived((now.getHours() + now.getMinutes() / 60 - ag.fromHour) * HOUR);

  function keepInView(el: HTMLElement, cur: boolean) {
    if (cur) queueMicrotask(() => el.scrollIntoView({ block: 'nearest' }));
    return { update(c: boolean) { if (c) el.scrollIntoView({ block: 'nearest' }); } };
  }
</script>

{#snippet item(r: TaskRow, time: string)}
  <span class="agtext"><span class:dim={!!time}>{time ? time + ' ' : ''}</span>{#if r.priority}<span class="{cls(r.priStyle)} bold">({r.priority})&nbsp;</span>{/if}{#each r.words ?? [] as w, i}{#if i > 0}&nbsp;{/if}<span class={cls(w.style)} class:done={w.style === 'done'} class:match={w.match}>{w.text}</span>{/each}</span>
{/snippet}

<div class="ag">
  <div class="line calhead">
    <button class="prompt" onclick={() => onweek(-1)} aria-label="previous week">‹</button>
    <span class="prompt">{ag.title}</span>
    <button class="prompt" onclick={() => onweek(1)} aria-label="next week">›</button>
    <span class="dim calcount">{ag.count === 1 ? '1 task' : `${ag.count} tasks`}</span>
  </div>

  <div class="agcols agtop">
    <div></div>
    {#each ag.days ?? [] as d}
      <div class="aghead" class:c-today={d.today} class:bold={d.today} class:dim={d.past && !d.today}>{d.label}</div>
    {/each}
    <div class="dim agallday-label">all day</div>
    {#each ag.days ?? [] as d}
      <div class="agallday" class:today={d.today}>
        {#each d.allDay ?? [] as it (it.row.index)}
          <div class="agitem" class:cur={it.row.cursor && focused} class:curdim={it.row.cursor && !focused} class:marked={it.row.marked}
            use:keepInView={it.row.cursor} onclick={(e) => onclick(it.row, e)} role="button" tabindex="-1">{@render item(it.row, '')}</div>
        {/each}
      </div>
    {/each}
  </div>

  <div class="agscroll">
    <div class="agcols" style="height: {height}lh">
      <div class="aggutter">
        {#each hours as h, i}<div class="aghour" style="top: {i * HOUR}lh"><span class="dim">{h}:00</span></div>{/each}
      </div>
      {#each ag.days ?? [] as d}
        <div class="agday" class:today={d.today}>
          {#each hours as _, i}<div class="agline" style="top: {i * HOUR}lh"></div>{/each}
          {#if d.today && nowTop >= 0 && nowTop <= height}<div class="agnow" style="top: {nowTop}lh"></div>{/if}
          {#each place(d.timed) as p (p.it.row.index)}
            <div class="agitem timed" style="top: {p.top}lh" class:cur={p.it.row.cursor && focused} class:curdim={p.it.row.cursor && !focused}
              class:marked={p.it.row.marked} use:keepInView={p.it.row.cursor} onclick={(e) => onclick(p.it.row, e)} role="button" tabindex="-1"
              title={p.it.time}>{@render item(p.it.row, p.it.time)}</div>
          {/each}
        </div>
      {/each}
    </div>
  </div>
</div>
