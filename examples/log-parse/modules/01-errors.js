register({
  name: "errors",
  tag: "errors",

  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    const level = String(obj.level || "").toUpperCase();
    if (level !== "ERROR" && level !== "FATAL") return null;

    return {
      level,
      message: obj.msg || obj.message || line,
      fields: {
        service: obj.service,
        trace_id: obj.trace_id,
      },
    };
  },
});

