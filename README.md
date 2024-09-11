# GOverbuild

> [!WARNING]
> GOverbuild is currently in version 0. Please expect major changes to function and method signatures, data structures, and package structures without regards for backwards compatibility.

GOverbuild is a Go library of implementations of common LEGO Universe data structures and encodings.

## Installation

You can use the following command after initializing your project's module:

```bash
go get -u github.com/I-Am-Dench/goverbuild@latest
```

## Tools

A small collection of analysis tools is included within the `goverbuild` program which can be installed with the following command:

```bash
go install github.com/I-Am-Dench/goverbuild/cmd/goverbuild@latest
```

More information can be found here: [`/cmd/goverbuild/README.md`](cmd/goverbuild/README.md)

## Contents

### `/archive`

Packages related to `.pk` (pack), `.pki` (catalog), and `.txt` (manifest) files.

**TODO:**
- [ ] pack writer
- [ ] catalog writer
- [ ] manifest writer
- [ ] pack, catalog, and manifest tests

### `/compress`

Packages related to `.sd0` and `.si0` files. Currently, ONLY `.sd0` reading is implemented. Both `.sd0` and `.si0` readers and writers will be implemented within the `/sid0` package.

**TODO:**
- [ ] sd0 writer
- [ ] si0 reader and writer
- [ ] sd0 and si0 tests

### `/encoding`

Packages related to encoding data structures. The `ldf` implementation was taken and modified from my original implementation at [https://github.com/I-Am-Dench/nimbus-launcher/tree/main/ldf](https://github.com/I-Am-Dench/nimbus-launcher/tree/e119aae79cb439e9fb199ce7efab7d86e5e43d73/ldf).

### `/models`

Packages of data structures with defined layouts. This only really exists as I personally needed it for a side project, but it may still be useful to others.

**TODO:**
- [ ] See [https://github.com/I-Am-Dench/goverbuild/blob/main/models/charxml/charxml.go#L1-L12](https://github.com/I-Am-Dench/goverbuild/blob/main/models/charxml/charxml.go#L1-L12)
- [ ] charxml tests

## References

1. [LU Documentation](https://docs.lu-dev.net/en/latest/)