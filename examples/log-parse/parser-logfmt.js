register({
  name: "example-logfmt",

  parse(line, ctx) {
    const obj = log.parseLogfmt(line);
    if (!obj) return null;

    return {
      timestamp: obj.ts,
      level: obj.level || "INFO",
      message: obj.msg || line,
      fields: obj,
    };
  },
});

