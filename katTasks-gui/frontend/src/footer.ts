// Splitting a footer line from the task list into buttons and text.

export interface SegLike {
  text: string;
  style?: string;
  action?: string;
}

export type Button = { action: string; key: string; label: string };
export type Piece<S extends SegLike> = Button | { segs: S[] };

export const isButton = (p: unknown): p is Button => !!p && typeof p === 'object' && 'action' in p;

// The key and label pieces of one action become a button; plain text stays
// text, except the " • " between two buttons, which is dropped.
export function pieces<S extends SegLike>(line: S[]): Piece<S>[] {
  const out: Piece<S>[] = [];
  for (const g of line) {
    const last = out[out.length - 1];
    if (g.action) {
      const b = isButton(last) && last.action === g.action ? last : { action: g.action, key: '', label: '' };
      if (b !== last) out.push(b);
      if (g.style === 'key') b.key += g.text;
      else b.label += g.text;
    } else if (last && !isButton(last)) {
      last.segs.push(g);
    } else {
      out.push({ segs: [g] });
    }
  }
  return out.filter((p, i) => isButton(p) || !p.segs.every((g) => g.text.trim() === '•')
    || !isButton(out[i - 1]) || !isButton(out[i + 1]));
}
