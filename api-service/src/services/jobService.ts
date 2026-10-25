import { randomUUID as uuidv4 } from 'node:crypto';
import pool from '../db/mysql';
import { CreateJobInput } from '../validators/jobValidator';
import { canonical, digest } from './identity';
import { Job } from '../types/job';

export class JobService {
  async createJob(input: CreateJobInput, requestKey?: string): Promise<Job> {
    const jobId = uuidv4();
    const keyHash = requestKey ? digest(requestKey) : null;
    const requestHash = requestKey ? digest(canonical(input)) : null;
    const conn = await pool.getConnection();
    try {
      await conn.beginTransaction();
      // S1: write into db (source of truth)
      await conn.execute(
        `INSERT INTO jobs (id, type, payload, status, max_retries, priority, request_key, request_hash)
        VALUES (?, ?, ?, 'PENDING', ?, ?, ?, ?)`,
        [
          jobId,
          input.type,
          JSON.stringify(input.payload),
          input.max_retries,
          input.priority,
          keyHash,
          requestHash,
        ],
      );

      // S2: write audit_log entry
      await conn.execute(
        `INSERT INTO job_audit_logs (job_id, from_status, to_status, message)
         VALUES (?, NULL, 'PENDING', 'Job created')`,
        [jobId],
      );

      await conn.execute(
        `INSERT INTO job_outbox (job_id, generation, attempt, available_at)
         VALUES (?, 1, 1, NOW(6))`,
        [jobId],
      );
      await conn.commit();

      // S4: Fetch & returned job
      const [created] = await conn.execute<any[]>(
        'SELECT * FROM jobs WHERE id = ?',
        [jobId],
      );
      const job = created[0] as Job;
      if (!job) throw new Error('Job created but cloud not be fetched');
      return job;
    } catch (err) {
      await conn.rollback();
      if (keyHash && (err as { code?: string }).code === 'ER_DUP_ENTRY') {
        const [rows] = await conn.execute<any[]>(
          'SELECT * FROM jobs WHERE request_key = ?',
          [keyHash],
        );
        if (rows.length) {
          if (rows[0].request_hash !== requestHash)
            throw new Error('IDEMPOTENCY_CONFLICT');
          return rows[0] as Job;
        }
      }
      throw err;
    } finally {
      conn.release();
    }
  }

  async getJobById(id: string): Promise<Job | null> {
    const [rows] = await pool.execute<any[]>(
      `SELECT * FROM jobs WHERE id = ?`,
      [id],
    );
    if (rows.length === 0) return null;

    const row = rows[0];
    return {
      ...row,
      payload:
        typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload,
    } as Job;
  }

  async listJobs(filters: {
    status?: string;
    type?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ jobs: Job[]; total: number }> {
    const { status, type, limit = 20, offset = 0 } = filters;
    const conditions: string[] = [];
    const params: unknown[] = [];

    if (status) {
      conditions.push('status = ?');
      params.push(status);
    }

    if (type) {
      conditions.push('type = ?');
      params.push(type);
    }

    const whereClause =
      conditions.length > 0 ? `WHERE ${conditions.join(' AND ')}` : '';

    const [rows] = await pool.query<any[]>(
      `SELECT * FROM jobs ${whereClause} ORDER BY created_at DESC LIMIT ? OFFSET ?`,
      [...params, limit, offset],
    );

    const [countRows] = await pool.query<any[]>(
      `SELECT COUNT(*) as total FROM jobs ${whereClause}`,
      params,
    );

    return {
      jobs: rows.map((row) => ({
        ...row,
        payload:
          typeof row.payload === 'string'
            ? JSON.parse(row.payload)
            : row.payload,
      })),
      total: countRows[0].total,
    };
  }

  async listDeadJobs(
    limit = 20,
    offset = 0,
  ): Promise<{ jobs: Job[]; total: number }> {
    const [rows] = await pool.execute<any[]>(
      `SELECT id, type, payload, status, retry_count, max_retries,
            priority, created_at, updated_at, processed_at, error_msg
     FROM jobs
     WHERE status = 'DEAD'
     ORDER BY updated_at DESC
     LIMIT ? OFFSET ?`,
      [limit, offset],
    );

    const [countRows] = await pool.execute<any[]>(
      `SELECT COUNT(*) as total FROM jobs WHERE status = 'DEAD'`,
    );

    return {
      jobs: rows.map((row) => ({
        ...row,
        payload:
          typeof row.payload === 'string'
            ? JSON.parse(row.payload)
            : row.payload,
      })),
      total: countRows[0].total,
    };
  }

  async requeueJob(id: string): Promise<Job> {
    const connection = await pool.getConnection();
    try {
      await connection.beginTransaction();

      const [rows] = await connection.execute<any[]>(
        'SELECT * FROM jobs WHERE id = ? FOR UPDATE',
        [id],
      );
      const job = rows[0];
      if (!job) throw new Error('JOB_NOT_FOUND');
      if (job.status !== 'DEAD' && job.status !== 'FAILED')
        throw new Error('JOB_NOT_REQUEUEABLE');
      // Lock and increment generation: old Redis messages cannot execute a replay.
      // Reset job state
      await connection.execute(
        `UPDATE jobs
       SET status = 'PENDING', retry_count = 0, attempt_count = 0,
           generation = generation + 1, next_attempt_at = NOW(6),
           lease_owner = NULL, lease_until = NULL, error_msg = NULL,
           processed_at = NULL, updated_at = NOW()
       WHERE id = ?`,
        [id],
      );

      await connection.execute(
        `INSERT INTO job_audit_logs (job_id, from_status, to_status, message)
       VALUES (?, ?, 'PENDING', 'Manually requeued via API')`,
        [id, job.status],
      );

      await connection.execute(
        `INSERT INTO job_outbox (job_id, generation, attempt, available_at)
         VALUES (?, ?, 1, NOW(6))`,
        [id, job.generation + 1],
      );
      await connection.commit();

      const [updatedRows] = await connection.execute<any[]>(
        'SELECT * FROM jobs WHERE id = ?',
        [id],
      );
      const updated = updatedRows[0] as Job;
      if (!updated) throw new Error('Failed to fetch updated job');
      return updated;
    } catch (error) {
      await connection.rollback();
      throw error;
    } finally {
      connection.release();
    }
  }

  async getJobLogs(id: string): Promise<any[]> {
    const [rows] = await pool.execute<any[]>(
      `SELECT id, job_id, from_status, to_status, message, worker_id, created_at
     FROM job_audit_logs
     WHERE job_id = ?
     ORDER BY created_at ASC`,
      [id],
    );
    return rows;
  }
}
