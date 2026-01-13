register({
  name: "example-json",

  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    return {
      timestamp: obj.ts,
      level: obj.level || "INFO",
      message: obj.msg || line,
      trace_id: obj.trace_id,
      service: obj.service,
    };
  },
});

