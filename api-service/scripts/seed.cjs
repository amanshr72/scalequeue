const mysql = require('mysql2/promise');
const { randomUUID } = require('node:crypto');
(async () => {
  if (!process.env.DEMO_API_KEY) throw new Error('DEMO_API_KEY required');
  const db = await mysql.createConnection({host:process.env.DB_HOST,port:Number(process.env.DB_PORT||3306),user:process.env.DB_USER,password:process.env.DB_PASSWORD,database:process.env.DB_NAME});
  try { await db.execute('INSERT INTO api_clients (id,name,api_key) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE is_active=TRUE',[randomUUID(),'Local demo',process.env.DEMO_API_KEY]); }
  finally { await db.end(); }
})().catch(e=>{console.error(e);process.exitCode=1;});
