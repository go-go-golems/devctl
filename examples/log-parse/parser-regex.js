register({
  name: "example-regex",

  parse(line, ctx) {
    // NOTE: goja RegExp does not support JS named capture groups (?<name>...),
    // so we use positional groups here.
    const m = /^(\w+)\s+\[([^\]]+)\]\s+(.*)$/.exec(line);
    if (!m) return null;

    return { level: m[1], service: m[2], message: m[3] };
  },
});
