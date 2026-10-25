// Requires a fresh disposable database. Exercises legacy upgrade and down/up.
const {execFileSync}=require('node:child_process');
const path=require('node:path');
const assert=require('node:assert/strict');
const mysql=require('../api-service/node_modules/mysql2/promise');
const cwd=path.resolve(__dirname,'../api-service');
function migrate(direction,count){execFileSync(process.execPath,['node_modules/db-migrate/bin/db-migrate',direction,...(count?['--count',String(count)]:[])],{cwd,stdio:'pipe',env:process.env});}
(async()=>{
 const db=await mysql.createConnection({host:process.env.DB_HOST,port:Number(process.env.DB_PORT),user:process.env.DB_USER,password:process.env.DB_PASSWORD,database:process.env.DB_NAME});
 try{
  const [tables]=await db.query("SHOW TABLES");assert.equal(tables.length,0,'migration test requires an EMPTY disposable database');
  migrate('up',3);
  for(const [id,status,retries] of [['upgrade-pending','PENDING',2],['upgrade-interrupted','PROCESSING',1]])await db.execute("INSERT INTO jobs(id,type,payload,status,retry_count,max_retries) VALUES(?,'send_email','{}',?,?,3)",[id,status,retries]);
  migrate('up');
  const [rows]=await db.query("SELECT j.id,j.status,j.attempt_count,o.attempt FROM jobs j JOIN job_outbox o ON o.job_id=j.id ORDER BY j.id");assert.equal(rows.length,2);for(const row of rows){assert.equal(row.status,'PENDING');assert.equal(row.attempt_count,2);assert.equal(row.attempt,3);}
  await db.query('DELETE FROM jobs');
  migrate('down',1);migrate('up');migrate('up');
  console.log('PASS: legacy retry budgets, interrupted-job recovery backfill, migration down/up, repeat up');
 }finally{await db.end()}
})().catch(e=>{console.error(e);process.exitCode=1});
