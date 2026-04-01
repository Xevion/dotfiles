# Platform Detection Reference

How to detect the current platform inside chezmoi templates, and when to use each variable.

## Available Variables

These are set in `home/.chezmoi.toml.tmpl` and available in all regular templates via `.` and in partials via `.data`:

| Variable | Type | Values | Source |
|---|---|---|---|
| `.chezmoi.os` | string | `"linux"`, `"windows"`, `"darwin"` | chezmoi built-in |
| `.chezmoi.arch` | string | `"amd64"`, `"arm64"`, etc. | chezmoi built-in |
| `.chezmoi.homeDir` | string | `/home/xevion`, `C:\Users\Xevion` | chezmoi built-in |
| `.chezmoi.hostname` | string | machine hostname | chezmoi built-in |
| `.chezmoi.kernel.osrelease` | string | kernel version string | chezmoi built-in (Linux only) |
| `.wsl` | bool | `true` / `false` | custom: detected from kernel osrelease |
| `.chassis` | string | `"laptop"` / `"desktop"` | custom: detected from DMI/WMI/sysctl |
| `.cpu.cores` | int | e.g. `8` | custom: physical cores |
| `.cpu.threads` | int | e.g. `16` | custom: logical threads |

## Platform Detection Logic

### OS Detection

`.chezmoi.os` is always available and is the primary OS discriminator:

```
{{- if eq .chezmoi.os "linux" }}
{{- if eq .chezmoi.os "darwin" }}
{{- if eq .chezmoi.os "windows" }}
```

### WSL Detection

WSL runs as Linux but the kernel osrelease string contains "microsoft":

```
{{- $wsl := false -}}
{{- if eq .chezmoi.os "linux" -}}
{{-   $wsl = (.chezmoi.kernel.osrelease | lower | contains "microsoft") -}}
{{- end -}}
```

The `.wsl` data variable is pre-computed in `.chezmoi.toml.tmpl`. Use it directly in templates:

```
{{- if .wsl }}
# Running in WSL
{{- end }}
```

### Chassis Detection

`.chassis` is `"laptop"` or `"desktop"`. Useful for battery/power settings.

| Platform | Detection method |
|---|---|
| Linux (native) | `/sys/class/dmi/id/chassis_type` (SMBIOS chassis codes) |
| WSL / Windows | `Win32_Battery` presence via PowerShell |
| macOS | `sysctl hw.model` contains "MacBook" |

SMBIOS chassis codes mapped to `"laptop"`: 8, 9, 10, 14 (portable, laptop, notebook, sub-notebook).

### CPU Detection

`.cpu.cores` and `.cpu.threads` are integers detected at apply time:

| Platform | Cores source | Threads source |
|---|---|---|
| Linux | `lscpu` "Core(s) per socket" | `nproc --all` |
| macOS | `sysctl hw.physicalcpu_max` | `sysctl hw.logicalcpu_max` |
| Windows | `Win32_Processor.NumberOfCores` | `Win32_Processor.NumberOfLogicalProcessors` |

## Standard Platform Conditional Patterns

### Full platform switch

```
{{- if and (eq .chezmoi.os "windows") (not .wsl) }}
# Windows-only (native, not WSL)
{{- else if .wsl }}
# WSL-only
{{- else if eq .chezmoi.os "linux" }}
# Native Linux
{{- else if eq .chezmoi.os "darwin" }}
# macOS
{{- end }}
```

### Linux + WSL combined (any Linux environment)

```
{{- if eq .chezmoi.os "linux" }}
# Both native Linux and WSL
{{- end }}
```

### Windows + WSL combined (any Windows context)

```
{{- if or (eq .chezmoi.os "windows") .wsl }}
# Either native Windows or WSL (has access to Windows tools)
{{- end }}
```

### Laptop-specific settings

```
{{- if eq .chassis "laptop" }}
# Battery/power management settings
{{- end }}
```

### Architecture-specific

```
{{- if eq .chezmoi.arch "arm64" }}
# Apple Silicon or ARM Linux
{{- end }}
```

## In Partial Templates

Partials (`home/.chezmoitemplates/`) receive context as a parameter. Access everything through `.data`:

```
{{- if and (eq .data.chezmoi.os "windows") (not .data.wsl) }}
{{- else if .data.wsl }}
{{- else if eq .data.chezmoi.os "linux" }}
{{- end }}
```

The calling template passes context with `{{ template "name.tmpl" . }}`.

## File-Level Exclusion

To include/exclude entire files rather than using conditionals inside them, use `.chezmoiignore`:

```
# home/.chezmoiignore

# Exclude AppData on non-Windows
{{- if ne .chezmoi.os "windows" }}
AppData/**
{{- end }}

# Exclude Windows Terminal config on non-Windows
{{- if ne .chezmoi.os "windows" }}
AppData/Local/Packages/**
{{- end }}
```

See `home/.chezmoiignore` for the full current list of conditional exclusions.

## Doppler Secrets in Templates

Secrets from Doppler are available via `dopplerProjectJson`:

```
{{ dopplerProjectJson.MY_SECRET }}
```

These are injected at apply time. The Doppler project is `dotfiles`, config `production`.
