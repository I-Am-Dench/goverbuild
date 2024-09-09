# GOverbuild Tools

This is collection of tools for analyzing and extracting `.pk`, `.pki`, and `.txt` data.

This program was intended for testing library functionality and analyzing pack and catalog data, but the [`extract`](#extract) command may be of use to some users.

## Installation

```bash
go install github.com/I-Am-Dench/goverbuild/cmd/goverbuild@latest
```

## Commands

### `pack`

- `show`: Display a table of a pack's records
- `dump`: Dump each file within a pack
- `search`: Find a specific record by a resource's path
- `extract`: Extract a specific file by a resource's path

### `catalog`

- `show`: Display a table of a catalog's packs
- `search`: Find a specific pack within a catalog

### `manifest`

Display a manifest file's version and/or resources.

### `extract`

Extracts **ALL** resources from a given manifest file and catalog file. This command is best used when extracting resources from a packed client. I would recommend anyone extracting a packed client to still use LCDR's [pkextractor](https://github.com/lcdr/utils) for those not familiar with the command line, but if you would like to use `goverbuild`'s extractor, I would recommend calling

```bash
goverbuild extract -v -ie
```

in your packed client's installation directory. That is, the directory containing the `versions`, `patcher`, `installer`, and `client` directories.