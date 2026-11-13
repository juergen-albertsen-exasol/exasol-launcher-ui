import { formatSql } from './format';

describe('formatSql', () => {
  it('upper-cases keywords and puts FROM and WHERE on lines of their own', () => {
    expect(formatSql('select a,b from t where a=1')).toBe(['SELECT a,b', 'FROM t', 'WHERE a=1'].join('\n'));
  });

  it('puts each top-level clause on its own line with uppercase keywords', () => {
    expect(
      formatSql(
        "select customer_id, sum(total_eur) as revenue from uploads.orders where status = 'open' group by customer_id order by 2 desc limit 10"
      )
    ).toBe(
      [
        'SELECT customer_id, SUM(total_eur) AS revenue',
        'FROM uploads.orders',
        "WHERE status = 'open'",
        'GROUP BY customer_id',
        'ORDER BY 2 DESC',
        'LIMIT 10',
      ].join('\n')
    );
  });

  it('leaves string literals, quoted identifiers, and comments unchanged', () => {
    expect(formatSql(`select 'from where' as "select", x -- keep select here\nfrom t`)).toBe(
      [`SELECT 'from where' AS "select", x -- keep select here`, 'FROM t'].join('\n')
    );
  });

  it("keeps subqueries on their clause's line", () => {
    expect(formatSql('select * from (select a from t where b = 1) s join u on s.a = u.a')).toBe(
      ['SELECT *', 'FROM (SELECT a FROM t WHERE b = 1) s', 'JOIN u ON s.a = u.a'].join('\n')
    );
  });
});
