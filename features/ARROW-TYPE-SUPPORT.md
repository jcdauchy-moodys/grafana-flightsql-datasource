# Arrow Data Type Support

This document describes the comprehensive Arrow data type support in the FlightSQL datasource plugin, with special attention to Oracle database types.

## Supported Arrow Types

### Numeric Types

| Arrow Type | Grafana Field Type | Handler | Oracle Types |
|-----------|-------------------|---------|--------------|
| `INT8` | `int8` | ✅ Basic | - |
| `INT16` | `int16` | ✅ Basic | - |
| `INT32` | `int32` | ✅ Basic | - |
| `INT64` | `int64` | ✅ Basic | - |
| `UINT8` | `uint8` | ✅ Basic | - |
| `UINT16` | `uint16` | ✅ Basic | - |
| `UINT32` | `uint32` | ✅ Basic | - |
| `UINT64` | `uint64` | ✅ Basic | - |
| `FLOAT32` | `float32` | ✅ Basic | `FLOAT`, `BINARY_FLOAT` |
| `FLOAT64` | `float64` | ✅ Basic | `NUMBER` (no precision), `BINARY_DOUBLE` |
| `DECIMAL128` | `string` | ✅ Custom | `NUMBER(p,s)` with precision |
| `DECIMAL256` | `string` | ✅ Custom | High-precision decimals |

### String Types

| Arrow Type | Grafana Field Type | Handler | Oracle Types |
|-----------|-------------------|---------|--------------|
| `STRING` | `string` | ✅ Basic | `VARCHAR2`, `NVARCHAR2`, `CHAR`, `NCHAR`, `CLOB`, `NCLOB`, `ROWID`, `UROWID` |

### Boolean Type

| Arrow Type | Grafana Field Type | Handler | Oracle Types |
|-----------|-------------------|---------|--------------|
| `BOOL` | `bool` | ✅ Basic | - |

### Date/Time Types

| Arrow Type | Grafana Field Type | Handler | Oracle Types |
|-----------|-------------------|---------|--------------|
| `TIMESTAMP` | `time.Time` | ✅ Custom | `TIMESTAMP`, `TIMESTAMP WITH TIME ZONE`, `TIMESTAMP WITH LOCAL TIME ZONE` |
| `DATE32` | `time.Time` | ✅ Custom | `DATE` |
| `DATE64` | `time.Time` | ✅ Custom | `DATE` (alternative) |
| `DURATION` | `int64` | ✅ Basic | - |

### Interval Types

| Arrow Type | Grafana Field Type | Handler | Oracle Types |
|-----------|-------------------|---------|--------------|
| `INTERVAL_MONTHS` | `int32` | ✅ Custom | `INTERVAL YEAR TO MONTH` |
| `INTERVAL_DAY_TIME` | `int64` | ✅ Custom | `INTERVAL DAY TO SECOND` |

### Binary Types

| Arrow Type | Grafana Field Type | Handler | Oracle Types |
|-----------|-------------------|---------|--------------|
| `BINARY` | `string` | ✅ Custom | `BLOB`, `RAW` |
| `LARGE_BINARY` | `string` | ✅ Custom | `LONG RAW` |
| `FIXED_SIZE_BINARY` | `string` | ✅ Custom | Fixed-size binary data |

### Special Types

| Arrow Type | Grafana Field Type | Handler | Oracle Types |
|-----------|-------------------|---------|--------------|
| `NULL` | `json.RawMessage` | ✅ Custom | NULL-only columns |
| `DENSE_UNION` | `json.RawMessage` | ✅ Custom | Union types |

### Fallback Handling

| Arrow Type | Grafana Field Type | Handler | Notes |
|-----------|-------------------|---------|-------|
| **Any other type** | `json.RawMessage` | ✅ Default | Logs warning, fills with null/empty values |

## Type Conversion Details

### Decimal Types (DECIMAL128, DECIMAL256)
Oracle `NUMBER(p,s)` types are converted to Arrow Decimal types and then to strings to preserve exact precision. This prevents floating-point rounding errors.

Example:
```sql
-- Oracle: NUMBER(10,2) = 123.45
-- Arrow: DECIMAL128(10,2) 
-- Grafana: "123.45" (string)
```

### Timestamp Types
All timestamp types are converted to `time.Time` with nanosecond precision. Timezone information is preserved when available.

Example:
```sql
-- Oracle: TIMESTAMP WITH TIME ZONE = '2024-01-15 10:30:00 +00:00'
-- Arrow: TIMESTAMP with microsecond precision
-- Grafana: time.Time object
```

### Date Types
Oracle `DATE` type (which includes time) is converted to Arrow `DATE32` and then to `time.Time`.

### Interval Types

#### INTERVAL_MONTHS (Oracle INTERVAL YEAR TO MONTH)
Stored as `int32` representing the number of months.

Example:
```sql
-- Oracle: INTERVAL '2-3' YEAR TO MONTH
-- Arrow: INTERVAL_MONTHS = 27 (2*12 + 3)
-- Grafana: 27 (int32)
```

#### INTERVAL_DAY_TIME (Oracle INTERVAL DAY TO SECOND)
Converted to nanoseconds and stored as `int64`.

Example:
```sql
-- Oracle: INTERVAL '2 10:30:00' DAY TO SECOND
-- Arrow: INTERVAL_DAY_TIME = {Days: 2, Milliseconds: 37800000}
-- Grafana: nanoseconds = (2*24*60*60*1000000000) + (37800000*1000000)
```

### Binary Types
Binary data is converted to string representation (as bytes). Large binary objects (BLOBs) are handled the same way.

### Unknown/Unsupported Types
When an unknown Arrow type is encountered:
1. A warning is logged with the type name and ID
2. The field is filled with appropriate null/empty values to maintain row alignment
3. All fields will have the same number of rows (prevents "different field lengths" error)

## Error Prevention

### Field Length Mismatch
Previous versions could produce the error:
```
frame has different field lengths, field 0 is len 295 but field 25 is len 0
```

This has been fixed by ensuring:
- Every type handler returns data for all rows
- Unknown types fill with null/empty values instead of skipping
- Explicit `return nil` after each case to prevent fall-through

## Oracle-Specific Considerations

### NUMBER Type Handling
- `NUMBER` without precision → `Float64`
- `NUMBER(p,s)` with precision → `Decimal128` → `string` (to preserve precision)

### ROWID and UROWID
Treated as regular strings since they're already string representations.

### LOB Types (CLOB, NCLOB, BLOB)
- Character LOBs → `String`
- Binary LOBs → `Binary` → `string` representation

### Unicode Types
`NVARCHAR2` and `NCHAR` are handled the same as their non-Unicode counterparts since Arrow strings are UTF-8 by default.

## Testing Recommendations

To verify type support, test queries with various Oracle types:

```sql
-- Test numeric types
SELECT 
  1 as int_val,
  1.5 as float_val,
  TO_NUMBER('123.456789012345678901234567890') as decimal_val
FROM DUAL;

-- Test date/time types
SELECT 
  SYSDATE as date_val,
  CURRENT_TIMESTAMP as timestamp_val,
  SYSTIMESTAMP as timestamp_tz_val
FROM DUAL;

-- Test interval types
SELECT 
  INTERVAL '2-3' YEAR TO MONTH as year_month_interval,
  INTERVAL '2 10:30:00' DAY TO SECOND as day_second_interval
FROM DUAL;

-- Test string types
SELECT 
  'varchar' as varchar_val,
  CAST('clob' AS CLOB) as clob_val,
  ROWID as rowid_val
FROM DUAL;
```

## Logging

When an unsupported type is encountered, check the Grafana logs for messages like:
```
Unsupported Arrow type <TypeName> (ID: <ID>) for field <FieldName>, filling with empty values
```

This helps identify any Oracle-specific types that may need special handling in future versions.

