
(function(){
  function parseJSON(line) {
    try { return JSON.parse(line); } catch (e) { return null; }
  }

  // Extremely small logfmt-ish parser: key=value pairs separated by spaces.
  // Supports quoted values with \" escapes.
  function parseLogfmt(line) {
    if (typeof line !== "string") return null;
    const out = {};
    let i = 0;
    const n = line.length;

    function skipSpaces() { while (i < n && line[i] === " ") i++; }
    function parseKey() {
      const start = i;
      while (i < n && line[i] !== "=" && line[i] !== " ") i++;
      if (i === start) return null;
      return line.slice(start, i);
    }
    function parseValue() {
      if (i >= n) return "";
      if (line[i] === "\"") {
        i++;
        let s = "";
        while (i < n) {
          const ch = line[i];
          if (ch === "\"") { i++; return s; }
          if (ch === "\\\\") {
            i++;
            if (i < n) { s += line[i]; i++; continue; }
            return s;
          }
          s += ch;
          i++;
        }
        return s;
      }
      const start = i;
      while (i < n && line[i] !== " ") i++;
      return line.slice(start, i);
    }

    while (i < n) {
      skipSpaces();
      if (i >= n) break;
      const key = parseKey();
      if (!key) break;
      if (i >= n || line[i] !== "=") { out[key] = true; continue; }
      i++; // skip '='
      const value = parseValue();
      out[key] = value;
      skipSpaces();
    }
    return out;
  }

  function parseKeyValue(line, delimiter, separator) {
    if (typeof line !== "string") return null;
    delimiter = (typeof delimiter === "string" && delimiter.length > 0) ? delimiter : " ";
    separator = (typeof separator === "string" && separator.length > 0) ? separator : "=";
    const parts = line.split(delimiter);
    const out = {};
    for (const part of parts) {
      const p = part.trim();
      if (p === "") continue;
      const idx = p.indexOf(separator);
      if (idx < 0) { out[p] = true; continue; }
      const k = p.slice(0, idx).trim();
      if (k === "") continue;
      let v = p.slice(idx + separator.length).trim();
      if (v.startsWith("\"") && v.endsWith("\"") && v.length >= 2) {
        v = v.slice(1, -1);
      }
      out[k] = v;
    }
    return out;
  }

  function namedCapture(line, re) {
    if (typeof line !== "string") return null;
    if (!(re instanceof RegExp)) return null;
    const m = re.exec(line);
    if (!m) return null;
    if (!m.groups) return null;
    const out = {};
    for (const k of Object.keys(m.groups)) out[k] = m.groups[k];
    return out;
  }

  function capture(line, re) {
    if (typeof line !== "string") return null;
    if (!(re instanceof RegExp)) return null;
    const m = re.exec(line);
    if (!m) return null;
    const out = [];
    for (let i = 1; i < m.length; i++) {
      out.push((m[i] === undefined) ? null : m[i]);
    }
    return out;
  }

  function extract(line, re, group) {
    if (typeof line !== "string") return null;
    if (!(re instanceof RegExp)) return null;
    const m = re.exec(line);
    if (!m) return null;
    const idx = (typeof group === "number") ? group : 1;
    const v = m[idx];
    return (typeof v === "string") ? v : null;
  }

  function getPath(obj, path) {
    if (!obj || typeof path !== "string" || path === "") return null;
    const parts = path.split(".");
    let cur = obj;
    for (const p of parts) {
      if (cur == null) return null;
      cur = cur[p];
    }
    return (cur === undefined) ? null : cur;
  }

  function hasPath(obj, path) {
    return getPath(obj, path) !== null;
  }

  function field(obj, path) {
    return getPath(obj, path);
  }

  function addTag(event, tag) {
    if (!event || typeof tag !== "string" || tag.trim() === "") return;
    tag = tag.trim();
    if (!Array.isArray(event.tags)) event.tags = [];
    for (const t of event.tags) if (t === tag) return;
    event.tags.push(tag);
  }

  function removeTag(event, tag) {
    if (!event || typeof tag !== "string" || tag.trim() === "") return;
    tag = tag.trim();
    if (!Array.isArray(event.tags)) return;
    event.tags = event.tags.filter(t => t !== tag);
  }

  function hasTag(event, tag) {
    if (!event || typeof tag !== "string" || tag.trim() === "") return false;
    tag = tag.trim();
    if (!Array.isArray(event.tags)) return false;
    for (const t of event.tags) if (t === tag) return true;
    return false;
  }

  function toNumber(value) {
    if (value == null) return null;
    if (typeof value === "number") return Number.isFinite(value) ? value : null;
    const s = String(value).trim();
    if (s === "") return null;
    const n = Number(s);
    return Number.isFinite(n) ? n : null;
  }

  function parseDurationMs(value) {
    if (value == null) return 0;
    if (typeof value === "number") return Number.isFinite(value) ? Math.max(0, Math.floor(value)) : 0;
    const s0 = String(value).trim();
    if (s0 === "") return 0;

    const m = s0.match(/^(\d+(?:\.\d+)?)(ms|s|m|h)$/);
    if (!m) return 0;
    const n = Number(m[1]);
    if (!Number.isFinite(n)) return 0;
    const unit = m[2];
    if (unit === "ms") return Math.max(0, Math.floor(n));
    if (unit === "s") return Math.max(0, Math.floor(n * 1000));
    if (unit === "m") return Math.max(0, Math.floor(n * 60 * 1000));
    if (unit === "h") return Math.max(0, Math.floor(n * 60 * 60 * 1000));
    return 0;
  }

  // Multiline buffer (single-threaded, deterministic).
  //
  // Supported config (MVP):
  // - pattern: RegExp (required)
  // - negate: boolean (default false)
  // - match: "after" (default). "before" is rejected for now.
  // - maxLines: number (default 200)
  // - timeout: duration string (e.g. "5s") (best-effort; flushes when next line arrives)
  //
  // add(line) -> string|null:
  // - returns a combined multi-line string when the buffer decides an event is complete
  // - returns null while still accumulating
  //
  // flush() -> string|null:
  // - returns any currently buffered content and clears the buffer
  function createMultilineBuffer(config) {
    config = config || {};
    const pattern = config.pattern;
    if (!(pattern instanceof RegExp)) throw new Error("createMultilineBuffer: config.pattern must be a RegExp");
    const negate = !!config.negate;
    const match = (typeof config.match === "string") ? config.match : "after";
    if (match !== "after") throw new Error("createMultilineBuffer: only match='after' is supported in this iteration");

    let maxLines = 200;
    if (typeof config.maxLines === "number" && Number.isFinite(config.maxLines) && config.maxLines > 0) {
      maxLines = Math.floor(config.maxLines);
    }

    const timeoutMs = parseDurationMs(config.timeout);
    let buf = [];
    let lastAddAt = 0;

    function flushInternal() {
      if (buf.length === 0) return null;
      const out = buf.join("\n");
      buf = [];
      return out;
    }

    function appendLine(line) {
      buf.push(line);
      if (buf.length >= maxLines) return flushInternal();
      return null;
    }

    return {
      add(line) {
        if (typeof line !== "string") line = String(line);
        const now = Date.now();
        if (timeoutMs > 0 && buf.length > 0 && lastAddAt > 0 && (now - lastAddAt) > timeoutMs) {
          const flushed = flushInternal();
          buf = [line];
          lastAddAt = now;
          return flushed;
        }
        lastAddAt = now;

        const isStart = negate ? !pattern.test(line) : pattern.test(line);
        if (isStart && buf.length > 0) {
          const flushed = flushInternal();
          buf = [line];
          return flushed;
        }
        return appendLine(line);
      },

      flush() {
        return flushInternal();
      }
    };
  }

  function parseTimestamp(value, formats) {
    // Go will override this with a more capable parser; keep a safe fallback.
    try {
      if (value == null) return null;
      if (value instanceof Date) return value;
      const d = new Date(value);
      return isNaN(d.getTime()) ? null : d;
    } catch (e) {
      return null;
    }
  }

  globalThis.log = {
    parseJSON,
    parseLogfmt,
    parseKeyValue,
    namedCapture,
    capture,
    extract,
    field,
    getPath,
    hasPath,
    addTag,
    removeTag,
    hasTag,
    parseTimestamp,
    toNumber,
    createMultilineBuffer,
  };
})();
