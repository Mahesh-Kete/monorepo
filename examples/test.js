const assert = require("node:assert");
const _ = require("lodash");

assert.strictEqual(_.startCase("hello world"), "Hello World");
console.log("tests passed");
