# POmerge

> [!WARNING]
> WIP: Tested on couple of small PO-files on Linux. Windows support pending. 

Git merge driver for PO-files.

- Provides a 3-way merge interface `pomerge a b c`.

## Installation

Install with `go install github.com/adventune/pomerge`.

## Usage

### Configure as merge driver

Git config:
```sh
git config merge.pomerge.driver "pomerge %A %O %B"
```

Also make sure that there is a following `.gitattributes` configuration:
```
[attr]POFILE merge=pomerge

*.po POFILE
```

### Executable

Run with: `pomerge a b c [output]`

### Library

Librarys public API:

- `pomerge.ThreeWay(a, b, c)`
- `pomerge.ThreeWayOut(a, b, c, out)`

Reference [Executable](./README.md#Params) for the param values
