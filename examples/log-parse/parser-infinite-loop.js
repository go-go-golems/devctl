register({
  name: "example-infinite-loop",
  parse(line, ctx) {
    while (true) {}
  },
});

