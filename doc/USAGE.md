# Usage

## Basic Usage

```bash
df-pv
```

## Usage with Columns

```bash
df-pv --columns "pv,size"
```

The inode columns (`iused`, `ifree`, and `%iused`) are available when explicitly selected, but are omitted from the default output.

Available columns:
- pv
- pvc
- namespace
- node
- pod
- mount
- size
- used
- available
- %used
- iused
- ifree
- %iused
