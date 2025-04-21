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