register({
  name: "example-regex",

  parse(line, ctx) {
    const m = log.namedCapture(line, /^(?<level>\\w+)\\s+\\[(?<service>[^\\]]+)\\]\\s+(?<msg>.*)$/);
    if (!m) return null;

    return {
      level: m.level,
      message: m.msg,
      service: m.service,
    };
  },
});

