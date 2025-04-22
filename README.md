# fluxbatcher

**fluxbatcher** is a command-line utility that helps generate and execute batch Flux queries based on a Markdown-defined value table and a query template. It is especially designed for migrating and transforming sensor data — for example, when moving historical Home Assistant data between InfluxDB buckets and adapting tag structures in the process. The total time range can optionally be split into smaller intervals (or "batches") so that queries are executed in manageable chunks. This helps avoid long execution times and makes it easier to track progress across the data range.

## 🔍 Purpose

This tool is particularly useful when:

- You want to **migrate sensor data** (e.g. from an old Home Assistant setup) to a new InfluxDB bucket.
- You need to **rename tags** like `entity_id`, or change `_measurement` or `domain` fields to match a new naming convention.
- You want to run the same Flux query repeatedly with varying values efficiently.

## 💡 How It Works

You provide:

- A **Flux query template** with placeholders (like `{{START}}`, `{{STOP}}`, which are derived from the command-line flags, and others such as `{{_measurement}}`, `{{domain}}`, `{{entity_id}}`, which typically refer to tags or fields in InfluxDB).
- A **Markdown table** that defines values to substitute into the template.

`fluxbatcher` renders one Flux query per row in the Markdown table, replaces placeholders, and prints them to `stdout` (or optionally writes them to files or executes them, in future versions).

## 📆 Installation

```bash
go install github.com/mfmayer/fluxbatcher@latest
```

Or build locally:

```bash
git clone https://github.com/mfmayer/fluxbatcher.git
cd fluxbatcher
go build -o fluxbatcher
```

## 🚀 Usage

```bash
fluxbatcher --template <template_file> --table <markdown_table> --start <start_time> --stop <stop_time> [--interval <duration>]
```

### Required flags:

- `--template`: Path to the Flux query template file.
- `--table`: Path to the Markdown value table.
- `--start`: Start time for the query (`{{START}}`).
- `--stop`: Stop time for the query (`{{STOP}}`).

### Optional:

- `--interval`: Chunk duration (e.g. `1h`, `12h`, `1d`, `1m`, `1y`). If set, the time range will be split into intervals, and the query will be repeated per interval and table row. Supported time units are `h` (hour), `d` (day), `m` (month), and `y` (year).

## 📄 Example

This example shows how to migrate sensor data from an old InfluxDB bucket (old\_homeassist) to a new one (new\_homeassist), while also transforming tag and measurement values. The goal is to align historical data with the tag format expected by a freshly installed Home Assistant instance.

### `examples/template.flux`:

```flux
import "date"

from(bucket: "old_homeassist")
  |> range(start: {{START}}, stop: {{STOP}})
  |> filter(fn: (r) => r.entity_id == "{{from}}")
  |> filter(fn: (r) => r._field == "value")
  |> map(fn: (r) => ({
      _time: date.truncate(t: r._time,unit: 1s),
      _value: r._value,
      _field: r._field,
      _measurement: "{{measurement}}",
      domain: "{{domain}}",
      entity_id: "{{to}}",
  }))
  |> sort(columns: ["_time"])  
  |> to(bucket: "new_homeassist")
```

### `examples/values.md`:

```markdown
| from                                   | to                                 | measurement         | domain |
| -------------------------------------- | ---------------------------------- | ------------------- | ------ |
| lumi_lumi_weather_8d7ab7e2_humidity    | lumi_klima_living_room_humidity    | sensor__humidity    | sensor |
| lumi_lumi_weather_8d7ab7e2_temperature | lumi_klima_living_room_temperature | sensor__temperature | sensor |
| lumi_lumi_weather_8d7ab7e2_pressure    | lumi_klima_living_room_pressure    | sensor__pressure    | sensor |
| lumi_lumi_weather_8d7ab7e2_power       | lumi_klima_living_room_power       | sensor__power       | sensor |
```

### Run it:

```bash
$ cd examples
$ fluxbatcher \
  --template template.flux \
  --table values.md \
  --start 2023-01-01T00:00:00Z \
  --stop 2024-01-01T00:00:00Z \
  --interval 1m
row 0: Processing: 2023-01-01T00:00:00Z → 2024-01-01T00:00:00Z [=================================================>] 100% (🕓 7s) ✅
row 1: Processing: 2023-01-01T00:00:00Z → 2024-01-01T00:00:00Z [=================================================>] 100% (🕓 3s) ✅
row 2: Processing: 2023-01-01T00:00:00Z → 2024-01-01T00:00:00Z [=================================================>] 100% (🕓 6s) ✅
row 3: Processing: 2023-01-01T00:00:00Z → 2024-01-01T00:00:00Z [=================================================>] 100% (🕓 5s) ✅
```

This will generate one Flux query per value row and per 1-month interval.

## 🤝 Contributing

PRs welcome — especially for features like direct query execution, output file support, or CSV table support.

## 📜 License

MIT License
