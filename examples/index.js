const chalk = require("chalk");
const _ = require("lodash");

const items = ["citadel", "ebpf", "runtime", "edr"];
console.log(chalk.cyan(_.startCase(items.join(" "))));
