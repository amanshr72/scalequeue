import { z } from "zod";

export const createJobSchema = z.object({
  type: z.enum([
    "send_email",
    "send_notification",
    "generate_report",
    "payment_retry",
    "fail_job",
  ]),
  payload: z
    // .record(z.unknown())
    .record(z.string(), z.unknown())
    .refine((val) => Object.keys(val).length > 0, {
      message: "Payload cannot be empty",
    }),
  max_retries: z.number().int().min(0).max(10).optional().default(3),
  priority: z.number().int().min(1).max(10).optional().default(5),
});

export type CreateJobInput = z.infer<typeof createJobSchema>;
