<script lang="ts">
  // The katTasks window. It draws the Snapshot the task list sends and
  // hands it every key press and click; all the behavior lives in Go, in
  // the same model the terminal app runs.
  import { onMount, tick } from 'svelte';
  import { Clipboard, Events } from '@wailsio/runtime';
  import * as api from '../bindings/github.com/devinthemtn/katTasks/katTasks-gui/tasksservice';
  import type { Snapshot, Seg, TaskRow, SideRow } from '../bindings/github.com/devinthemtn/katTasks/internal/ui/models';
  import { teaKey, isPaste, zoomStep } from './keys';
  import { pieces, isButton } from './footer';
  import Text from './Text.svelte';
  import TaskLine from './TaskLine.svelte';
  import CalendarView from './Calendar.svelte';
  import AgendaView from './Agenda.svelte';

  let s = $state<Snapshot | null>(null);
  let startupError = $state('');
  let dark = $state(matchMedia('(prefers-color-scheme: dark)').matches);
  // gui.font_size and gui.font_family from the settings; ctrl+= / ctrl+- zoom
  // from that size for this session, ctrl+0 goes back to it.
  let zoomBy = $state(0);
  const MIN_SIZE = 8, MAX_SIZE = 48;
  const baseSize = $derived(s?.fontSize || 14);
  const fontSize = $derived(Math.min(MAX_SIZE, Math.max(MIN_SIZE, baseSize + zoomBy)));
  const font = $derived(s?.font ?? '');

  let measure: HTMLSpanElement;
  let listEl = $state<HTMLDivElement>();
  let cw = 8; // character width and row height, in px
  let rh = 18;

  // ---- talking to Go ------------------------------------------------------

  // Calls run one at a time, in order, so fast typing can't be reordered.
  let queue: Promise<unknown> = Promise.resolve();
  function send(call: () => Promise<Snapshot>) {
    queue = queue.then(call).then(apply, (err) => console.error(err));
  }
  function apply(n: Snapshot) {
    if (!n || (s && n.seq <= s.seq)) return;
    s = n;
  }

  const key = (k: string) => send(() => api.Key(k));
  const run = (action: string) => send(() => api.Run(action));

  // ---- size ---------------------------------------------------------------

  function remeasure() {
    if (!measure) return;
    const r = measure.getBoundingClientRect();
    cw = r.width / 10 || 8;
    rh = r.height || 18;
    resize();
  }

  function resize() {
    if (!listEl) return;
    const cols = Math.floor(window.innerWidth / cw);
    const rows = Math.max(1, Math.floor(listEl.clientHeight / rh));
    send(() => api.Resize(cols, rows));
  }

  function zoom(step: number) {
    zoomBy = step === 0 ? 0 : Math.min(MAX_SIZE, Math.max(MIN_SIZE, fontSize + step)) - baseSize;
  }

  // A new font or size changes how many characters and rows fit.
  $effect(() => {
    void fontSize; void font;
    tick().then(remeasure);
  });

  // ---- keys and mouse ------------------------------------------------------

  const typing = $derived(!!s?.input);

  async function onKeydown(e: KeyboardEvent) {
    const z = zoomStep(e);
    if (z !== null) {
      e.preventDefault();
      zoom(z);
      return;
    }
    if (isPaste(e)) {
      e.preventDefault();
      if (typing) {
        const text = await Clipboard.Text().catch(() => '');
        if (text) send(() => api.Paste(text));
      }
      return;
    }
    const k = teaKey(e);
    if (k === null) return;
    e.preventDefault(); // tab, space, ctrl+r, / … belong to the task list
    if (s?.help && scrollHelp(k)) return;
    key(k);
  }

  // The help pane scrolls with these keys when it doesn't all fit; any
  // other key goes to the task list, which closes it.
  let helpEl: HTMLElement | undefined = $state();
  function scrollHelp(k: string): boolean {
    const el = helpEl;
    if (!el || el.scrollHeight <= el.clientHeight) return false;
    const half = Math.max(rh, Math.floor(el.clientHeight / 2));
    const by: Record<string, number> = {
      j: rh, down: rh, k: -rh, up: -rh,
      pgdown: half, ' ': half, 'ctrl+d': half, pgup: -half, 'ctrl+u': -half,
      g: -el.scrollHeight, home: -el.scrollHeight, G: el.scrollHeight, end: el.scrollHeight,
    };
    if (!(k in by)) return false;
    el.scrollBy({ top: by[k] });
    return true;
  }

  let wheel = 0;
  function onWheel(e: WheelEvent, sidebar: boolean) {
    e.preventDefault();
    wheel += e.deltaMode === 1 ? e.deltaY * rh : e.deltaY;
    const n = Math.trunc(wheel / rh);
    if (n !== 0) {
      wheel -= n * rh;
      send(() => api.Scroll(n, sidebar));
    }
  }

  function clickTask(r: TaskRow, e: MouseEvent) {
    send(() => api.ClickTask(r.index, e.detail >= 2, false));
  }
  function toggleTask(r: TaskRow, e: MouseEvent) {
    e.stopPropagation();
    if (e.detail < 2) send(() => api.ClickTask(r.index, false, true));
  }
  function clickSide(r: SideRow) {
    if (r.kind !== 'header') send(() => api.ClickSidebar(r.index));
  }
  const pick = (i: number, commit: boolean) => send(() => api.Pick(i, commit));

  // Clicking in an input moves its cursor to the clicked character.
  function clickInput(e: MouseEvent) {
    const el = e.currentTarget as HTMLElement;
    const r = el.getBoundingClientRect();
    const pos = Math.round((e.clientX - r.left + el.scrollLeft) / cw);
    send(() => api.SetInputCursor(Math.max(0, pos)));
  }

  // ---- drawing helpers ------------------------------------------------------

  const colors = $derived(s ? (dark ? s.colors.dark : s.colors.light) : {});
  const fallback: Record<string, [string, string]> = {
    fg: ['#1F1F24', '#E6E6EA'], dim: ['#8A8A8A', '#6E6E7A'], bg: ['#FFFFFF', '#18181F'],
  };
  const cssVars = $derived(
    Object.entries({ ...Object.fromEntries(Object.keys(fallback).map((k) => [k, ''])), ...colors })
      .map(([k, v]) => `--${k.replace('_', '-')}: ${v || (fallback[k] ? fallback[k][dark ? 1 : 0] : 'inherit')}`)
      .join('; ') + `; font-size: ${fontSize}px` + (font ? `; font-family: ${font}` : ''),
  );

  const cols = $derived(Math.floor(window.innerWidth / cw));
  const sideW = $derived(Math.min(32, Math.max(20, Math.floor(cols / 4))));

  // The input's text split around the cursor, for drawing a block cursor.
  function caret(value: string, pos: number) {
    const r = [...value];
    return { before: r.slice(0, pos).join(''), at: r[pos] ?? ' ', after: r.slice(pos + 1).join('') };
  }

  // Keep the cursor of a long input in view.
  function followCaret(el: HTMLElement) {
    $effect(() => {
      void s?.input?.pos; void s?.input?.value;
      const c = el.querySelector('.caret') as HTMLElement | null;
      if (!c) return;
      // Where the cursor is in the text, in px from its start. (offsetLeft
      // would count from the popup's overlay, not from this box.)
      const x = c.getBoundingClientRect().left - el.getBoundingClientRect().left + el.scrollLeft;
      const margin = Math.min(cw * 4, el.clientWidth / 4);
      if (x + c.offsetWidth > el.scrollLeft + el.clientWidth - margin) el.scrollLeft = x + c.offsetWidth - el.clientWidth + margin;
      else if (x < el.scrollLeft + margin) el.scrollLeft = Math.max(0, x - margin);
    });
  }

  function scrollIntoView(el: HTMLElement, selected: boolean) {
    if (selected) el.scrollIntoView({ block: 'nearest' });
    return {
      update(sel: boolean) { if (sel) el.scrollIntoView({ block: 'nearest' }); },
    };
  }

  const popupHelp: Seg[][] = [
    [{ text: '(A) ', style: 'key' }, { text: 'priority, at the start', style: 'dim' }],
    [{ text: '+project ', style: 'project' }, { text: '@context ', style: 'context' }, { text: 'key:value', style: 'tag' }],
    [{ text: 'due:', style: 'tag' }, { text: 'opens a calendar, or type today fri +3d 2026-12-01', style: 'dim' }],
    [{ text: 'rec:', style: 'tag' }, { text: '1w 3d 5b 1m 1y · rec:+1w counts from due', style: 'dim' }],
    [{ text: 'alarm:', style: 'tag' }, { text: 'opens a date and time picker; rings like an alarm clock', style: 'dim' }],
    [{ text: 'at:', style: 'tag' }, { text: '930 1400 3pm · on its due: day, or daily; agenda + alerts', style: 'dim' }],
    [{ text: 'status:doing progress:40 ', style: 'tag' }, { text: 'in progress · i > < in the list', style: 'dim' }],
    [{ text: '/name ', style: 'key' }, { text: 'run a Lua script from scripts/', style: 'dim' }],
  ];

  onMount(() => {
    const mq = matchMedia('(prefers-color-scheme: dark)');
    const onScheme = () => (dark = mq.matches);
    mq.addEventListener('change', onScheme);
    const off = Events.On('kattasks:state', (ev) => apply(ev.data as Snapshot));
    const ro = new ResizeObserver(() => resize());

    (async () => {
      startupError = await api.StartupError();
      if (startupError) return;
      apply(await api.State());
      await tick();
      remeasure();
      if (listEl) ro.observe(listEl);
    })();

    return () => {
      mq.removeEventListener('change', onScheme);
      off();
      ro.disconnect();
    };
  });
</script>

<svelte:window onkeydown={onKeydown} onresize={resize} />

<div class="app" style={cssVars}>
  <span class="measure" bind:this={measure} aria-hidden="true">0123456789</span>

  {#if startupError}
    <div class="startup"><p class="error">{startupError}</p><p class="dim">Fix the problem, then start katTasks again.</p></div>
  {:else if s}
    <!-- header -->
    <div class="header line">
      <span class="title">katTasks</span>
      <span class="dim path"><span dir="ltr">{s.path}</span></span>
      <span class="dim info">{s.info}</span>
    </div>

    <!-- body: sidebar and list -->
    <div class="body">
      <div class="pane side" class:focused={s.focus === 'sidebar'} style="width: {sideW}ch"
        onwheel={(e) => onWheel(e, true)}>
        {#each s.side ?? [] as r (r.index)}
          {#if r.kind === 'header'}
            <div class="line dim bold sidehead">&nbsp;{r.label}</div>
          {:else}
            <div class="line siderow {r.kind}" class:cur={r.cursor} class:hl={r.cursor && s.focus === 'sidebar'}
              onclick={() => clickSide(r)} role="button" tabindex="-1">
              <span class="marker">{r.cursor ? '▌' : ' '}</span><span class="label">{r.label}</span><span class="dim count">{r.count}</span>
            </div>
          {/if}
        {/each}
      </div>

      <div class="pane main" class:focused={s.focus === 'list'} bind:this={listEl}
        onwheel={(e) => { if (s?.mode === 'normal' && !s.help && !s.calendar && !s.agenda) onWheel(e, false); }}>
        {#if s.mode === 'palette' && s.input}
          <div class="pad">
            <div class="line"><span class="prompt">{': '}</span><span class="input" use:followCaret onclick={clickInput} role="textbox" tabindex="-1">{caret(s.input.value, s.input.pos).before}<span class="caret">{caret(s.input.value, s.input.pos).at}</span>{caret(s.input.value, s.input.pos).after}</span></div>
            <div class="line">&nbsp;</div>
            <div class="scroll">
              {#if !s.palette?.length}
                <div class="line dim">no matching command</div>
              {/if}
              {#each s.palette ?? [] as p, i}
                <div class="line palrow" class:sel={i === s.palIdx} use:scrollIntoView={i === s.palIdx}
                  onclick={() => pick(i, true)} role="button" tabindex="-1">
                  <span class="marker prompt">{i === s.palIdx ? '▌ ' : '  '}</span><span class="name" class:dim={p.disabled} class:bold={i === s.palIdx}>{p.name}</span><span class="dim desc">{p.desc}</span><span class="keys bold">{p.keys}</span>
                </div>
              {/each}
            </div>
          </div>
        {:else if s.mode === 'theme'}
          <div class="pad">
            <div class="line dim bold">THEME</div>
            <div class="line">&nbsp;</div>
            {#each s.themes ?? [] as t, i}
              <div class="line themerow" onclick={(e) => pick(i, e.detail >= 2)} role="button" tabindex="-1">
                {#if i === s.pickIdx}<span class="prompt">▌ {t.name}</span>{:else}<span>&nbsp;&nbsp;{t.name}</span>{/if}{#if t.file}<span class="dim">&nbsp;&nbsp;{t.file}</span>{/if}
              </div>
            {/each}
            <div class="line">&nbsp;</div>
            <div class="line dim">j/k preview · enter save · esc cancel</div>
            <div class="line dim">katTasks -maketheme NAME copies the current theme to edit</div>
          </div>
        {:else if s.help}
          <div class="pad scroll" bind:this={helpEl} onclick={() => key('esc')} role="button" tabindex="-1">
            <div class="line"><span class="dim bold">KEYS</span><span class="dim">&nbsp;&nbsp;&nbsp;katTasks {s.version}</span></div>
            <div class="line">&nbsp;</div>
            <div class="helpcols">
              {#each s.help as col}
                <div class="helpcol">
                  <div class="line dim bold">{col.title}</div>
                  {#each col.rows ?? [] as r}
                    <div class="line helprow" onclick={(e) => { e.stopPropagation(); run(r.action); }} role="button" tabindex="-1">
                      <span class="bold keys">{r.keys}</span><span class="dim">{r.desc}</span>
                    </div>
                  {/each}
                </div>
              {/each}
            </div>
            <div class="line">&nbsp;</div>
            <div class="line dim bold">WHILE TYPING</div>
            <div class="line">&nbsp;</div>
            {#each s.helpTyping ?? [] as l}<div class="line dim pre">{l}</div>{/each}
            <div class="line">&nbsp;</div>
            <div class="line dim bold">TAGS &amp; SEARCH</div>
            <div class="line">&nbsp;</div>
            {#each s.helpTags ?? [] as l}<div class="line dim pre">{l}</div>{/each}
            <div class="line">&nbsp;</div>
            <div class="line dim">j/k ↑↓ pgup/pgdn g/G scroll · any other key closes</div>
          </div>
        {:else if s.agenda}
          <AgendaView ag={s.agenda} focused={s.focus === 'list'} onclick={(r, e) => clickTask(r, e)}
            onweek={(d) => send(() => api.AgendaWeek(d))} />
        {:else if s.calendar}
          <CalendarView cal={s.calendar} onpick={pick} onmonth={(d) => send(() => api.CalendarMonth(d))}
            ontask={(i, open) => send(() => api.ClickCalendarTask(i, open))} />
        {:else if s.total === 0}
          <div class="empty dim">{s.empty}</div>
        {:else}
          {#each s.rows ?? [] as r (r.index)}
            <TaskLine {r} numW={s.numW} cur={r.cursor && s.focus === 'list'} marked={r.marked}
              onclick={(e) => clickTask(r, e)} ontoggle={(e) => toggleTask(r, e)} />
          {/each}
        {/if}
      </div>
    </div>

    <!-- footer -->
    <div class="footer">
      {#if (s.mode === 'filter' || s.mode === 'savename') && s.input}
        <div class="line"><span class="prompt">&nbsp;{s.input.label} ›&nbsp;</span><span class="input grow" use:followCaret onclick={clickInput} role="textbox" tabindex="-1">{caret(s.input.value, s.input.pos).before}<span class="caret">{caret(s.input.value, s.input.pos).at}</span>{caret(s.input.value, s.input.pos).after}</span></div>
        <div class="line">
          {#if s.input.sugg?.length}
            &nbsp;{#each s.input.sugg as g, i}<span class={i === s.input.suggIdx ? 'chip' : 'dim sugg'} onclick={() => pick(i, true)} role="button" tabindex="-1">{i === s.input.suggIdx ? g : ` ${g} `}</span>{/each}<span class="dim">&nbsp;&nbsp;&nbsp;tab complete · ↑↓ choose</span>
          {:else}
            <span class="dim">&nbsp;{s.input.hint}</span>
          {/if}
        </div>
      {:else if (s.mode === 'add' || s.mode === 'edit') && s.input}
        <div class="line">&nbsp;<span class="prompt">{s.mode === 'add' ? 'add task' : 'edit task'}</span></div>
        <div class="line btnline">{@render popupButtons()}</div>
      {:else}
        {#each s.footer as l}
          {@const ps = pieces(l ?? [])}
          <div class="line" class:btnline={ps.some(isButton)}>{#if ps.length}&nbsp;{#each ps as p}{#if isButton(p)}<button class="btn" onclick={() => run(p.action)}><span>{p.label.trim()}</span>{#if p.key}<kbd>{p.key}</kbd>{/if}</button>{:else}<Text segs={p.segs} />{/if}{/each}{/if}</div>
        {/each}
      {/if}
    </div>

    {#snippet popupButtons()}
      <button class="btn pos on" onclick={() => key('enter')}><span>Save</span><kbd>enter</kbd></button>
      <button class="btn neg" onclick={() => key('esc')}><span>Cancel</span><kbd>esc</kbd></button>
      <button class="btn" onclick={() => key('tab')}><span>Complete</span><kbd>tab</kbd></button>
      <span class="dim">&nbsp;<span class="bold">←→↑↓</span> choose</span>
    {/snippet}

    <!-- add / edit popup -->
    {#if (s.mode === 'add' || s.mode === 'edit') && s.input}
      <div class="overlay" onclick={() => key('esc')} role="presentation">
        <div class="pane focused popup" onclick={(e) => e.stopPropagation()} role="dialog" aria-label={s.input.label}>
          <div class="line prompt">{s.input.label}</div>
          <div class="line">&nbsp;</div>
          <div class="line"><span class="prompt">›&nbsp;</span><span class="input grow" use:followCaret onclick={clickInput} role="textbox" tabindex="-1">{caret(s.input.value, s.input.pos).before}<span class="caret">{caret(s.input.value, s.input.pos).at}</span>{caret(s.input.value, s.input.pos).after}</span></div>
          <div class="line">
            {#each s.input.sugg ?? [] as g, i}<span class={i === s.input.suggIdx ? 'chip' : 'dim sugg'} onclick={() => pick(i, true)} role="button" tabindex="-1">{i === s.input.suggIdx ? g : ` ${g} `}</span>{/each}{#if s.input.suggDesc}<span class="dim">&nbsp;&nbsp;{s.input.suggDesc}</span>{/if}&nbsp;
          </div>
          <div class="rule"></div>
          {#if s.input.alarm}
            {@const a = s.input.alarm}
            <div class="line"><span class="prompt bold">{a.kind === 'due' ? '📅 DUE' : '⏰ ALARM'}</span>&nbsp;&nbsp;<span>{a.when}</span>&nbsp;&nbsp;<span class="dim">{a.until}</span></div>
            <div class="line">&nbsp;</div>
            <div class="alarm-pick">
              <div class="alarm-cal">
                <div class="alarm-month">
                  <button class="dim" onclick={() => { if (a.field !== 'date') key('tab'); key('['); }} aria-label="previous month">‹</button>
                  <span class="prompt">{a.month}</span>
                  <button class="dim" onclick={() => { if (a.field !== 'date') key('tab'); key(']'); }} aria-label="next month">›</button>
                </div>
                <div class="alarm-grid">
                  {#each a.weekdays ?? [] as w}<span class="dim">{w}</span>{/each}
                  {#each a.days ?? [] as d, i}
                    <span class={d.selected ? (a.field === 'date' ? 'chip' : 'sugg-on') : d.today ? 'c-today' : !d.inMonth || d.past ? 'dim' : ''}
                      onclick={() => pick(i, false)} ondblclick={() => pick(i, true)} role="button" tabindex="-1">{d.day}</span>
                  {/each}
                </div>
              </div>
              {#if a.kind !== 'due'}
              <div class="alarm-time">
                <div class="dim">Time</div>
                <button class="dim" onclick={() => { if (a.field !== 'time') key('tab'); key('up'); }} aria-label="later">▲</button>
                <span class={a.field === 'time' ? 'chip' : 'sugg-on'} onclick={() => { if (a.field !== 'time') key('tab'); }} role="button" tabindex="-1">{a.time}</span>
                <button class="dim" onclick={() => { if (a.field !== 'time') key('tab'); key('down'); }} aria-label="earlier">▼</button>
                {#if a.typed}<div class="dim">typed {a.typed}</div>{/if}
              </div>
              {/if}
            </div>
            <div class="line">&nbsp;</div>
            <div class="line">
              {#if a.kind === 'due'}
                <Text segs={[{ text: '←→↑↓', style: 'key' }, { text: ' day · ', style: 'dim' }, { text: '[ ]', style: 'key' }, { text: ' month · or keep typing ', style: 'dim' },
                  { text: 'fri', style: 'key' }, { text: ' ', style: 'dim' }, { text: '+3d', style: 'key' }]} />
              {:else if a.field === 'date'}
                <Text segs={[{ text: '←→↑↓', style: 'key' }, { text: ' day · ', style: 'dim' }, { text: '[ ]', style: 'key' }, { text: ' month · ', style: 'dim' },
                  { text: 't', style: 'key' }, { text: ' today · ', style: 'dim' }, { text: 'T', style: 'key' }, { text: ' tomorrow', style: 'dim' }]} />
              {:else}
                <Text segs={[{ text: '↑↓', style: 'key' }, { text: ' 15 min · ', style: 'dim' }, { text: '←→', style: 'key' }, { text: ' hour · or type ', style: 'dim' },
                  { text: '6:45', style: 'key' }, { text: ' / ', style: 'dim' }, { text: '7pm', style: 'key' }]} />
              {/if}
            </div>
            <div class="line btnline">
              <button class="btn pos on" onclick={() => key('enter')}><span>{a.kind === 'due' ? 'Set due date' : 'Set alarm'}</span><kbd>enter</kbd></button>
              <button class="btn neg" onclick={() => key('esc')}><span>Cancel</span><kbd>esc</kbd></button>
              <span class="dim">&nbsp;tab date / time</span>
            </div>
          {:else if s.input.choice}
            {@const c = s.input.choice}
            <div class="line prompt">{c.prompt}</div>
            <div class="line">&nbsp;</div>
            {#each c.options ?? [] as o, i}
              <div class="line choicerow" class:sel={i === c.idx} onclick={() => pick(i, true)} role="button" tabindex="-1"><span class="dim">&nbsp;{i + 1}&nbsp;&nbsp;</span>{o}</div>
            {/each}
            <div class="line">&nbsp;</div>
            <div class="line btnline">
              <button class="btn pos on" onclick={() => key('enter')}><span>Pick</span><kbd>enter</kbd></button>
              <button class="btn neg" onclick={() => key('esc')}><span>Cancel</span><kbd>esc</kbd></button>
              <span class="dim">&nbsp;<span class="bold">↑↓</span> choose · <span class="bold">1–{c.options?.length ?? 0}</span> or first letter picks</span>
            </div>
          {:else}
            {#each popupHelp as l}<div class="line"><Text segs={l} /></div>{/each}
            <div class="line">&nbsp;</div>
            <div class="line btnline">{@render popupButtons()}</div>
          {/if}
        </div>
      </div>
    {/if}

    <!-- y/n question -->
    {#if s.mode === 'confirm'}
      <div class="overlay" role="presentation">
        <div class="pane focused popup confirm" role="alertdialog" aria-label={s.confirm}>
          <div class="line prompt wrap">{s.confirm}</div>
          <div class="line">&nbsp;</div>
          <div class="line btnline">
            <button class="btn pos" class:on={!s.confirmNo} onclick={() => key('y')}><span>Yes</span><kbd>y</kbd></button>
            <button class="btn neg" class:on={s.confirmNo} onclick={() => key('n')}><span>No</span><kbd>n</kbd></button>
            <span class="dim">&nbsp;<span class="bold">←→</span> choose · <span class="bold">enter</span> ok</span>
          </div>
        </div>
      </div>
    {/if}

    <!-- priority letters -->
    {#if s.mode === 'priority'}
      <div class="prichips">
        {#each ['A', 'B', 'C', 'D', 'E'] as p}
          <button class="c-pri-{p === 'E' ? 'other' : p.toLowerCase()}" onclick={() => key(p)}>({p})</button>
        {/each}
        <button class="dim" onclick={() => key(' ')}>clear</button>
      </div>
    {/if}
  {/if}
</div>
