# batrat

A lightweight terminal power profiler for Linux. Samples CPU time per process,
converts it to estimated power draw, and shows a live dashboard of what's
draining your machine. Will also run headless and output to a report.

## How it works

Every tick (default 1s) batrat reads `/proc/stat` and `/proc/[pid]/stat`,
computes per-process CPU jiffie deltas, and attributes system power
proportionally:

```
P_sys  = P_idle + cpu_util × (P_max − P_idle) × (f / f_max)
P_proc = (proc_cpu_delta / total_cpu_delta) × P_sys
```

The `(f / f_max)` term scales by the CPU's current frequency, so the estimate
tracks reality on any machine without per-machine tuning. Energy is integrated
over time, so reports are valid for any run length. No root required. Tune
`-idle-w` / `-max-w` to match your machine.

## Build

```
go build ./cmd/batrat
```

## Usage

### Interactive (live TUI)

```
./batrat
```

The dashboard shows a **system power area chart** at the top — a gradient fill
(`█` solid at the bottom fading to `░` at the top) that aligns with the text
block, neither wider than the `TREND` column nor extending left of `RANK`. Below
it, a ranked process table with a per-process **trend indicator** (`▲` rising /
`▼` falling / `▬` flat). The layout adapts to the terminal size and repaints
cleanly on resize.

- `q` / `ctrl+c` — quit and print report
- `space` — pause
- `r` — reset counters
- `s` — toggle ranking: `power` (per-tick draw) / `energy` (cumulative buildup)
- `+` / `-` — adjust sampling interval

### Daemon (headless, self-detaching)

```
./batrat -d -t 15        # 15 minutes, report → batrat-<timestamp>.txt
./batrat -d -t 1h30m -o report.csv -f csv
```

The process detaches immediately; sampling continues in the background and
the report is written when the duration elapses.

### Flags

| flag         | default | description                                  |
|--------------|---------|----------------------------------------------|
| `-d`         | off     | daemon mode                                  |
| `-t`         | `15m`   | duration: minutes (`15`) or Go duration (`1h30m`) |
| `-o`         |         | report output path (daemon: `batrat-<timestamp>.txt`) |
| `-f`         | `text`  | `text` or `csv`                              |
| `-interval`  | `1s`    | sampling interval                            |
| `-idle-w`    | `8`     | system idle power (W), heuristic fallback     |
| `-max-w`     | `105`   | system max power (W), heuristic fallback      |
| `-top`       | `15`    | processes in report                          |
| `-no-rapl`   | off     | force heuristic estimate, ignore RAPL         |

## Power sources

batrat uses the most accurate source available:

- **RAPL** (`src: rapl`) — reads real CPU package energy counters from
  `/sys/class/powercap` when readable (usually requires root). Measured watts.
- **Heuristic** (`src: est`) — estimates from CPU time, scaled by per-core
  frequency: `P = idle + util × (max − idle) × (f / f_max)`. Used automatically
  when RAPL is unavailable. The header shows which source is active.

## Report example

```
batrat report
  started   2026-10-02 18:30:00
  ended     2026-10-02 18:45:00
  duration  15m0s
  interval  1s
  power     idle 8.0 W, max 105.0 W
  total     450.21 Wh

  RANK  NAME                  PID      CPU%   AVG W   ENERGY     SHARE
  1     firefox               12345    45.2   23.1    12.34 Wh   38.2%
  2     Xorg                  4242     12.4   6.2     3.41 Wh    10.5%
```
