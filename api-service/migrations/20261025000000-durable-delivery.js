'use strict';
const fs = require('node:fs/promises');
const path = require('node:path');
async function apply(db, direction) {
  const sql = await fs.readFile(path.join(__dirname, 'sqls', `20261025000000-durable-delivery-${direction}.sql`), 'utf8');
  // This migration contains simple DDL/DML only (no stored programs or quoted semicolons).
  for (const statement of sql.split(';').map(x => x.trim()).filter(Boolean)) {
    await db.runSql(statement);
  }
}
exports.setup = function () {};
exports.up = db => apply(db, 'up');
exports.down = db => apply(db, 'down');
exports._meta = { version: 1 };
