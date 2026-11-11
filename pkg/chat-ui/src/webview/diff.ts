export type DiffRow =
  | { kind: 'ctx' | 'add' | 'del'; text: string; oldLine?: number; newLine?: number }
  | { kind: 'hunk'; text: string };

export function computeDiffRows(oldText: string, newText: string): DiffRow[] {
  const a = oldText === '' ? [] : oldText.replace(/\r\n/g, '\n').split('\n');
  const b = newText === '' ? [] : newText.replace(/\r\n/g, '\n').split('\n');

  let start = 0;
  while (start < a.length && start < b.length && a[start] === b[start]) start++;
  let endA = a.length - 1;
  let endB = b.length - 1;
  while (endA >= start && endB >= start && a[endA] === b[endB]) {
    endA--;
    endB--;
  }

  const middleA = a.slice(start, endA + 1);
  const middleB = b.slice(start, endB + 1);
  const ops: Array<{ kind: ' ' | '-' | '+'; text: string }> = [];
  for (let i = 0; i < start; i++) ops.push({ kind: ' ', text: a[i] });

  const n = middleA.length;
  const m = middleB.length;
  if (n > 0 || m > 0) {
    if (n * m <= 250000) {
      const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
      for (let i = n - 1; i >= 0; i--) {
        for (let j = m - 1; j >= 0; j--) {
          dp[i][j] = middleA[i] === middleB[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
        }
      }
      let i = 0;
      let j = 0;
      while (i < n && j < m) {
        if (middleA[i] === middleB[j]) {
          ops.push({ kind: ' ', text: middleA[i] });
          i++;
          j++;
        } else if (dp[i + 1][j] >= dp[i][j + 1]) {
          ops.push({ kind: '-', text: middleA[i++] });
        } else {
          ops.push({ kind: '+', text: middleB[j++] });
        }
      }
      while (i < n) ops.push({ kind: '-', text: middleA[i++] });
      while (j < m) ops.push({ kind: '+', text: middleB[j++] });
    } else {
      for (const line of middleA) ops.push({ kind: '-', text: line });
      for (const line of middleB) ops.push({ kind: '+', text: line });
    }
  }
  for (let i = endA + 1; i < a.length; i++) ops.push({ kind: ' ', text: a[i] });

  const full: DiffRow[] = [];
  let oldLine = 1;
  let newLine = 1;
  for (const op of ops) {
    if (op.kind === ' ') {
      full.push({ kind: 'ctx', text: op.text, oldLine: oldLine++, newLine: newLine++ });
    } else if (op.kind === '-') {
      full.push({ kind: 'del', text: op.text, oldLine: oldLine++ });
    } else {
      full.push({ kind: 'add', text: op.text, newLine: newLine++ });
    }
  }

  const changed = full.map((row, index) => (row.kind === 'ctx' ? -1 : index)).filter((index) => index >= 0);
  if (!changed.length) return [];

  const keep = new Set<number>();
  for (const index of changed) {
    for (let i = Math.max(0, index - 3); i <= Math.min(full.length - 1, index + 3); i++) keep.add(i);
  }

  const out: DiffRow[] = [];
  let index = 0;
  while (index < full.length) {
    if (!keep.has(index)) {
      index++;
      continue;
    }
    let end = index;
    while (end < full.length && keep.has(end)) end++;
    const group = full.slice(index, end);
    const olds = group.filter((row) => row.kind !== 'hunk' && row.oldLine !== undefined) as Array<{ oldLine: number }>;
    const news = group.filter((row) => row.kind !== 'hunk' && row.newLine !== undefined) as Array<{ newLine: number }>;
    const oldStart = olds[0]?.oldLine ?? 0;
    const newStart = news[0]?.newLine ?? 0;
    out.push({ kind: 'hunk', text: `@@ -${oldStart},${olds.length} +${newStart},${news.length} @@` });
    out.push(...group);
    index = end;
  }
  return out;
}
