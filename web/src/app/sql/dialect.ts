/** Exasol SQL's keywords, types, and built-in functions, for highlighting, completion, and formatting. */

export const KEYWORDS = `
absolute add after all alter analyze and any array as asc at authorization begin between by call cascade case cast check close
cluster column columns comment commit connect connection constraint constraints create cross current current_date current_schema
current_session current_statement current_timestamp current_user cursor database default delete desc describe distinct distribute
do drop else elseif elsif end enforce escape except execute exists explain export false fetch file first flush following for
foreign force from full function grant group grouping having identified identity if import in index inner insert intersect
interval into is join key language last left level like limit local lock merge minus natural nologging not null nulls of on open
or order outer over partition password preceding preferring primary priority profile range recompress refresh rename reorganize
replace restrict return returns revoke right role rollback rows schema schemas script select session set sql statement
system table tables then to transaction trigger true truncate unbounded union unique update user using values view virtual when
where window with without zone
`
  .trim()
  .split(/\s+/);

export const TYPES = `
bigint bool boolean char character date dec decimal double float geometry hashtype int integer number numeric precision real
smallint timestamp tinyint varchar varchar2 utf8 ascii
`
  .trim()
  .split(/\s+/);

export const FUNCTIONS = `
abs acos add_days add_hours add_minutes add_months add_seconds add_weeks add_years approximate_count_distinct ascii asin atan atan2
avg bit_and bit_check bit_length bit_lrotate bit_lshift bit_not bit_or bit_rrotate bit_rshift bit_set bit_to_num bit_xor
case cast ceil ceiling char_length character_length chr coalesce colognephonetic concat connect_by_iscycle connect_by_isleaf
connect_by_root convert convert_tz corr cos cosh cot count covar_pop covar_samp cume_dist current_cluster current_date
current_schema current_session current_statement current_timestamp current_user date_trunc day days_between dbtimezone
decode degrees dense_rank div dump edit_distance every exp extract first_value floor from_posix_time greatest group_concat
grouping hash_md5 hash_sha1 hash_sha256 hash_sha512 hash_tiger hashtype_md5 hour hours_between initcap insert instr is_boolean
is_date is_dsinterval is_number is_timestamp is_yminterval json_extract json_value lag last_value lcase lead least left length
listagg ln localtimestamp locate log log10 log2 lower lpad ltrim max median mid min minute minutes_between mod month
months_between now nproc nth_value ntile nullif nullifzero numtodsinterval numtoyminterval nvl nvl2 octet_length
percent_rank percentile_cont percentile_disc pi posix_time power radians rand random rank ratio_to_report regexp_instr
regexp_replace regexp_substr regr_avgx regr_avgy regr_count regr_intercept regr_r2 regr_slope regr_sxx regr_sxy regr_syy repeat
replace reverse right round row_number rowid rpad rtrim scope_user second seconds_between session_parameter sessiontimezone sign
sin sinh soundex space sqrt st_area st_distance st_intersects st_x st_y stddev stddev_pop stddev_samp substr substring sum
sys_connect_by_path sys_guid sysdate systimestamp tan tanh to_char to_date to_dsinterval to_number to_timestamp to_yminterval
translate trim trunc truncate typeof ucase unicode unicodechr upper user username value2proc var_pop var_samp variance week
weekday year years_between zeroifnull
`
  .trim()
  .split(/\s+/);

/** Words the formatter starts a new line with, longest first so `GROUP BY` wins over `GROUP`. */
export const CLAUSES = [
  'UNION ALL',
  'LEFT OUTER JOIN',
  'RIGHT OUTER JOIN',
  'FULL OUTER JOIN',
  'LEFT JOIN',
  'RIGHT JOIN',
  'FULL JOIN',
  'INNER JOIN',
  'CROSS JOIN',
  'GROUP BY',
  'ORDER BY',
  'PARTITION BY',
  'DISTRIBUTE BY',
  'SELECT',
  'FROM',
  'WHERE',
  'HAVING',
  'QUALIFY',
  'LIMIT',
  'UNION',
  'EXCEPT',
  'MINUS',
  'INTERSECT',
  'JOIN',
  'WITH',
  'VALUES',
  'SET',
  'INSERT INTO',
  'UPDATE',
  'DELETE FROM',
  'MERGE INTO',
  'USING',
  'WHEN',
];
