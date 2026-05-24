const nodeMajor = Number.parseInt(process.versions.node.split(".")[0] ?? "", 10);

if (!Number.isInteger(nodeMajor) || nodeMajor < 24 || nodeMajor >= 26) {
  console.error(
    [
      `Unsupported Node.js ${process.version}.`,
      "GoGoMail frontends are pinned to Node.js 24 LTS.",
      "Node.js 26 exposes Tailwind CSS DEP0205 warnings until Tailwind replaces module.register().",
      "Run `fnm use` from the repository root, or install/use Node.js 24.16.0.",
    ].join("\n"),
  );
  process.exit(1);
}
