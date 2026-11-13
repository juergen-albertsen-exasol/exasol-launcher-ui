import { CLAUSES, FUNCTIONS, KEYWORDS, TYPES } from './dialect';

type Token = { kind: 'word' | 'string' | 'quoted' | 'comment' | 'line-comment' | 'space' | 'other'; text: string };

const UPPER = new Set([...KEYWORDS, ...TYPES, ...FUNCTIONS].map(word => word.toUpperCase()));
const CLAUSE_WORDS = CLAUSES.map(clause => clause.split(' '));

function tokenize(sql: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < sql.length) {
    const rest = sql.slice(i);
    let match: RegExpExecArray | null;
    if ((match = /^\s+/.exec(rest))) tokens.push({ kind: 'space', text: match[0] });
    else if ((match = /^--[^\n]*/.exec(rest))) tokens.push({ kind: 'line-comment', text: match[0] });
    else if ((match = /^\/\*[\s\S]*?(\*\/|$)/.exec(rest))) tokens.push({ kind: 'comment', text: match[0] });
    else if ((match = /^'(?:[^']|'')*'?/.exec(rest))) tokens.push({ kind: 'string', text: match[0] });
    else if ((match = /^"(?:[^"]|"")*"?/.exec(rest))) tokens.push({ kind: 'quoted', text: match[0] });
    else if ((match = /^[A-Za-z_][A-Za-z0-9_$#]*/.exec(rest))) tokens.push({ kind: 'word', text: match[0] });
    else tokens.push({ kind: 'other', text: sql[i]! });
    i += (match?.[0] ?? sql[i]!).length;
  }
  return tokens;
}

/**
 * Lays out SQL with one top-level clause per line and keywords, types, and function names in uppercase.
 * Identifiers, quoted identifiers, string literals, and comments keep their text.
 */
export function formatSql(sql: string): string {
  const words = tokenize(sql.trim()).map(token =>
    token.kind === 'word' && UPPER.has(token.text.toUpperCase()) ? { ...token, text: token.text.toUpperCase() } : token
  );
  let out = '';
  let depth = 0;
  let pendingSpace = false;
  let index = 0;

  const meaningful = (from: number) => {
    const found: Token[] = [];
    for (let j = from; j < words.length && found.length < 3; j++) if (words[j]!.kind !== 'space') found.push(words[j]!);
    return found;
  };

  while (index < words.length) {
    const token = words[index]!;
    if (token.kind === 'space') {
      pendingSpace = out.length > 0;
      index++;
      continue;
    }
    if (depth === 0 && token.kind === 'word') {
      const ahead = meaningful(index);
      const clause = CLAUSE_WORDS.find(parts =>
        parts.every((part, k) => ahead[k]?.kind === 'word' && ahead[k]!.text.toUpperCase() === part)
      );
      if (clause && out.length > 0 && !out.endsWith('\n')) {
        out = out.trimEnd() + '\n';
        pendingSpace = false;
      }
    }
    if (
      pendingSpace &&
      !out.endsWith('\n') &&
      !(
        token.text === ',' ||
        token.text === ')' ||
        token.text === ';' ||
        out.endsWith('(') ||
        token.text === '.' ||
        out.endsWith('.')
      )
    ) {
      out += ' ';
    }
    pendingSpace = false;
    out += token.text;
    if (token.text === '(') depth++;
    else if (token.text === ')') depth = Math.max(0, depth - 1);
    if (token.kind === 'line-comment') out += '\n';
    index++;
  }
  return out.trimEnd();
}
