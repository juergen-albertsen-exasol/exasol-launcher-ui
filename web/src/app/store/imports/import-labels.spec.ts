import { formatBytes, formatCount, formatSeconds, queryStatement } from './import-labels';

describe('import labels', () => {
  it('formats counts, seconds, and sizes', () => {
    expect(formatCount(5_972_150)).toBe('5,972,150');
    expect(formatSeconds(13.44)).toBe('13.4 s');
    expect(formatSeconds(12)).toBe('12 s');
    expect(formatBytes(512)).toBe('512 bytes');
    expect(formatBytes(98_400_000)).toBe('98.4 MB');
  });

  it('queries an imported table with quoted names', () => {
    expect(queryStatement('DATA', 'YELLOW_TRIPDATA_2024')).toBe(
      'SELECT * FROM "DATA"."YELLOW_TRIPDATA_2024" LIMIT 100;'
    );
    expect(queryStatement('DATA', 'my "table"')).toBe('SELECT * FROM "DATA"."my ""table""" LIMIT 100;');
  });
});
