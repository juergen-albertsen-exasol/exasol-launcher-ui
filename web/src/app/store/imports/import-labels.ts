const NUMBER = new Intl.NumberFormat('en-US');
const SECONDS = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });
const BYTE_UNITS = ['bytes', 'kB', 'MB', 'GB', 'TB'];
const KILO = 1000;

export const formatCount = (count: number): string => NUMBER.format(count);

export const formatSeconds = (seconds: number): string => `${SECONDS.format(seconds)} s`;

export const formatBytes = (bytes: number): string => {
  let value = bytes;
  let unit = 0;
  while (value >= KILO && unit < BYTE_UNITS.length - 1) {
    value /= KILO;
    unit++;
  }
  return `${unit === 0 ? value : SECONDS.format(value)} ${BYTE_UNITS[unit]}`;
};

const quote = (identifier: string) => `"${identifier.replace(/"/g, '""')}"`;

/** The query that the SQL editor receives for an imported table. */
export const queryStatement = (schema: string, table: string): string =>
  `SELECT * FROM ${quote(schema)}.${quote(table)} LIMIT 100;`;
