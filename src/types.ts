import {DataQuery, DataSourceJsonData} from '@grafana/data'
import {formatSQL} from './components/sqlFormatter'

export interface SQLQuery extends DataQuery {
  queryText?: string
  format?: string
  rawEditor?: boolean
  table?: string
  columns?: string[]
  wheres?: string[]
  orderBy?: string
  groupBy?: string
  limit?: string
}

export const DEFAULT_QUERY: Partial<SQLQuery> = {}

/**
 * These are options configured for each DataSource instance
 */
export interface FlightSQLDataSourceOptions extends DataSourceJsonData {
  host?: string
  token?: string
  secure?: boolean
  username?: string
  password?: string
  selectedAuthType?: string
  metadata?: any
  databaseType?: string
  healthCheckQuery?: string
}

export interface SecureJsonData {
  password?: string
  token?: string
}

export type TablesResponse = {
  tables: string[]
}

export type ColumnsResponse = {
  columns: string[]
}

export const authTypeOptions = [
  {key: 0, label: 'none', value: 'none'},
  {key: 1, label: 'username/password', value: 'username/password'},
  {key: 2, label: 'token', value: 'token'},
]

export const databaseTypeOptions = [
  {key: 0, label: 'Generic', value: 'generic'},
  {key: 1, label: 'Oracle', value: 'oracle'},
  {key: 2, label: 'PostgreSQL', value: 'postgresql'},
  {key: 3, label: 'MySQL', value: 'mysql'},
  {key: 4, label: 'SQLite', value: 'sqlite'},
  {key: 5, label: 'DuckDB', value: 'duckdb'},
  {key: 6, label: 'ClickHouse', value: 'clickhouse'},
]

export const defaultHealthCheckQueries: Record<string, string> = {
  generic: 'SELECT 1',
  oracle: 'SELECT 1 FROM DUAL',
  postgresql: 'SELECT 1',
  mysql: 'SELECT 1',
  sqlite: 'SELECT 1',
  duckdb: 'SELECT 1',
  clickhouse: 'SELECT 1',
}

export const sqlLanguageDefinition = {
  id: 'sql',
  formatter: formatSQL,
}

export enum QueryFormat {
  Timeseries = 'time_series',
  Table = 'table',
}

export const QUERY_FORMAT_OPTIONS = [
  {label: 'Time series', value: QueryFormat.Timeseries},
  {label: 'Table', value: QueryFormat.Table},
]
