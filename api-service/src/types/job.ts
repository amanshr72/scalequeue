export type JobStatus =
  | "PENDING"
  | "PROCESSING"
  | "COMPLETED"
  | "FAILED"
  | "DEAD";

export type JobType =
  | "send_email"
  | "send_notification"
  | "generate_report"
  | "payment_retry";

export interface Job {
  id: string;
  type: JobType;
  payload: Record<string, unknown>;
  status: JobStatus;
  retry_count: number;
  max_retries: number;
  created_at: Date;
  updated_at: Date;
  processed_at: Date | null;
  error_msg: string | null;
}

export interface CreateJobDTO {
  type: JobType;
  payload: Record<string, unknown>;
  max_retries?: number;
}
