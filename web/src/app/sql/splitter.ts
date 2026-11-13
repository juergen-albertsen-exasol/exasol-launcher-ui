/** One statement of the editor's text, without its terminating semicolon. */
export interface Statement {
  text: string;
  /** Offset of the statement's first character in the editor's text. */
  from: number;
  /** Offset just after the statement's last character, before any semicolon. */
  to: number;
  /** Offset just after the terminating semicolon, or `to` when there is none. */
  end: number;
}

/**
 * Splits SQL text into statements at semicolons, skipping semicolons inside string literals, quoted
 * identifiers, and `--` and `/* *\/` comments. Statements that hold only whitespace and comments are left out.
 */
export function splitStatements(text: string): Statement[] {
  const statements: Statement[] = [];
  let start = 0;
  let meaningful = false;
  let i = 0;

  const push = (to: number, end: number) => {
    if (meaningful) {
      let from = start;
      while (from < to && /\s/.test(text[from]!)) from++;
      let last = to;
      while (last > from && /\s/.test(text[last - 1]!)) last--;
      statements.push({ text: text.slice(from, last), from, to: last, end });
    }
    start = end;
    meaningful = false;
  };

  while (i < text.length) {
    const char = text[i]!;
    const next = text[i + 1];
    if (char === '-' && next === '-') {
      const newline = text.indexOf('\n', i);
      i = newline === -1 ? text.length : newline + 1;
    } else if (char === '/' && next === '*') {
      const close = text.indexOf('*/', i + 2);
      i = close === -1 ? text.length : close + 2;
    } else if (char === "'" || char === '"') {
      meaningful = true;
      i = skipQuoted(text, i, char);
    } else if (char === ';') {
      push(i, i + 1);
      i++;
    } else {
      if (!/\s/.test(char)) meaningful = true;
      i++;
    }
  }
  push(text.length, text.length);
  return statements;
}

/** The offset after a quoted string or identifier starting at `start`; a doubled quote is part of it. */
function skipQuoted(text: string, start: number, quote: string): number {
  let i = start + 1;
  while (i < text.length) {
    if (text[i] === quote) {
      if (text[i + 1] === quote) i += 2;
      else return i + 1;
    } else i++;
  }
  return text.length;
}

/**
 * The statement that ⌘/Ctrl+Enter runs for a cursor at `offset`: the one that contains it, else the one
 * ending earlier on the same line, else the next one, else the last one before it.
 */
export function statementAt(statements: Statement[], offset: number, text: string): Statement | undefined {
  const containing = statements.find(statement => offset >= statement.from && offset <= statement.end);
  if (containing) return containing;
  const lineStart = text.lastIndexOf('\n', offset - 1) + 1;
  const before = statements.filter(statement => statement.end <= offset);
  const previous = before.at(-1);
  if (previous && previous.end >= lineStart) return previous;
  return statements.find(statement => statement.from >= offset) ?? previous;
}

/**
 * The editor offset of a 1-based line and column inside a statement, as database errors name them.
 * Positions past the statement's end are clamped to it.
 */
export function positionInStatement(statement: Pick<Statement, 'text' | 'from'>, line: number, column: number): number {
  let offset = 0;
  for (let current = 1; current < line; current++) {
    const newline = statement.text.indexOf('\n', offset);
    if (newline === -1) return statement.from + statement.text.length;
    offset = newline + 1;
  }
  const lineEnd = statement.text.indexOf('\n', offset);
  const lineLength = (lineEnd === -1 ? statement.text.length : lineEnd) - offset;
  return statement.from + offset + Math.min(Math.max(column - 1, 0), lineLength);
}

/** The line and column that an Exasol error message names, such as `[line 1, column 29]`. */
export function errorPosition(message: string): { line: number; column: number } | undefined {
  const match = /\[line (\d+), column (\d+)\]/i.exec(message);
  return match ? { line: Number(match[1]), column: Number(match[2]) } : undefined;
}

/** The first keyword of a statement, uppercased, after leading comments. */
export function firstKeyword(sql: string): string {
  const stripped = sql.replace(/^(\s|--[^\n]*(\n|$)|\/\*[\s\S]*?\*\/)*/, '');
  return /^[A-Za-z_]+/.exec(stripped)?.[0].toUpperCase() ?? '';
}
