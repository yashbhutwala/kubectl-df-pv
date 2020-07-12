
# Usage

The following assumes you have the plugin installed via

```shell
> curl https://krew.sh/df-pv | bash
> # . ~/.bashrc   # run if you use bash shell
> # . ~/.zshrc    # run if you use zsh shell
> kubectl df-pv
```

## Flags

```shell
> kubectl df-pv --help
df-pv

Usage:
  df-pv [flags]

Flags:
  -h, --help               help for df-pv
  -n, --namespace string   if present, the namespace scope for this CLI request (default is all namespaces)
  -v, --verbosity string   log level; one of [info, debug, trace, warn, error, fatal, panic] (default "info")
```
