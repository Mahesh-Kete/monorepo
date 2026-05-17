import chalk from 'chalk';
import _ from 'lodash';

const greeting = _.capitalize('hello, guardrail');
console.log(chalk.green(greeting));
