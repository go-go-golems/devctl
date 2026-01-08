register({
  name: "metrics",
  tag: "metrics",

  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    const durationMs = log.toNumber(obj.duration_ms);
    if (durationMs == null) return null;

    return {
      level: "INFO",
      message: "request_duration_ms",
      fields: {
        service: obj.service,
        route: obj.route,
        duration_ms: durationMs,
      },
    };
  },
});

