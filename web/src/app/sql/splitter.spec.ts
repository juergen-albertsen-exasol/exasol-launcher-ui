import { errorPosition, firstKeyword, positionInStatement, splitStatements, statementAt } from './splitter';

const texts = (sql: string) => splitStatements(sql).map(statement => statement.text);

describe('splitStatements', () => {
  it('splits at semicolons and trims each statement', () => {
    expect(texts('SELECT 1;\n\n  SELECT 2 ;')).toEqual(['SELECT 1', 'SELECT 2']);
  });

  it('keeps a last statement without a semicolon', () => {
    expect(texts('SELECT 1; SELECT 2')).toEqual(['SELECT 1', 'SELECT 2']);
  });

  it('skips semicolons in strings, quoted identifiers, and comments', () => {
    const sql = [
      "SELECT 'a;b', 'it''s; fine' FROM \"T;1\"; -- a comment; still a comment",
      '/* block; comment */ SELECT "x""y;";',
    ].join('\n');
    expect(texts(sql)).toEqual([
      "SELECT 'a;b', 'it''s; fine' FROM \"T;1\"",
      '-- a comment; still a comment\n/* block; comment */ SELECT "x""y;"',
    ]);
  });

  it('leaves out statements of only whitespace and comments', () => {
    expect(texts(';;  -- nothing\n; SELECT 1;\n/* end */')).toEqual(['SELECT 1']);
  });

  it("records each statement's offsets in the text", () => {
    const sql = 'SELECT 1;\n  SELECT 2;';
    const [first, second] = splitStatements(sql);
    expect(sql.slice(first!.from, first!.to)).toBe('SELECT 1');
    expect(first!.end).toBe(9);
    expect(sql.slice(second!.from, second!.to)).toBe('SELECT 2');
  });
});

describe('statementAt', () => {
  const sql = 'SELECT 1;\n\nSELECT 2;';
  const statements = splitStatements(sql);

  it("finds the statement on the cursor's line", () => {
    expect(statementAt(statements, sql.indexOf('SELECT 2') + 3, sql)?.text).toBe('SELECT 2');
    expect(statementAt(statements, sql.length, sql)?.text).toBe('SELECT 2');
    expect(statementAt(statements, 0, sql)?.text).toBe('SELECT 1');
  });

  it("takes the statement that ends on the cursor's line, after its semicolon", () => {
    expect(statementAt(splitStatements('SELECT 1;   \nSELECT 2;'), 11, 'SELECT 1;   \nSELECT 2;')?.text).toBe(
      'SELECT 1'
    );
  });

  it('takes the next statement from an empty line between two', () => {
    expect(statementAt(statements, 10, sql)?.text).toBe('SELECT 2');
  });
});

describe('error positions', () => {
  it('reads the line and column from an Exasol message', () => {
    expect(errorPosition('object TOTAL not found [line 1, column 29] (Session: 1)')).toEqual({ line: 1, column: 29 });
    expect(errorPosition('syntax error')).toBeUndefined();
  });

  it("maps a statement's line and column to an editor offset", () => {
    const sql = 'SELECT 1;\nSELECT CUSTOMER_ID,\n  SUM(TOTAL) FROM T;';
    const statement = splitStatements(sql)[1]!;
    const offset = positionInStatement(statement, 2, 7);
    expect(sql.slice(offset, offset + 5)).toBe('TOTAL');
  });

  it('clamps positions past the end', () => {
    const statement = { text: 'SELECT X', from: 10 };
    expect(positionInStatement(statement, 1, 99)).toBe(18);
    expect(positionInStatement(statement, 4, 1)).toBe(18);
  });
});

describe('firstKeyword', () => {
  it('skips leading comments', () => {
    expect(firstKeyword('-- make it\n/* now */ create table T (A INT)')).toBe('CREATE');
    expect(firstKeyword('  open schema S1')).toBe('OPEN');
  });
});
