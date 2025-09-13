# GOverbuild FDB Converter

This tool is for converting FDB files to/from a database (i.e. a relational database).

## Installation

```bash
go install github.com/I-Am-Dench/goverbuild/cmd/gb-fdb@latest
```

## Commands

### `toFdb`

Converts a database to an FDB file. An optional table name can be included to exclude specific tables or columns from the conversion.

### `fromFdb`

Converts an FDB file to a database. This command typically removes all tables within the database before creating the new tables stored in the FDB file. Because of this, the most common use case for this command is for initialization.

## Supported Drivers

- `sqlite3`