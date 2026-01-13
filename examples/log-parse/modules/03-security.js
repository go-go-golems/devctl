register({
  name: "security",
  tag: "security",

  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    const msg = String(obj.msg || obj.message || "");
    if (!msg.toLowerCase().includes("authentication failed")) return null;

    const ev = {
      level: "WARN",
      message: msg,
      fields: {
        service: obj.service,
        user: obj.user,
        ip: obj.ip,
      },
      tags: [],
    };

    log.addTag(ev, "auth_failed");
    return ev;
  },
});

