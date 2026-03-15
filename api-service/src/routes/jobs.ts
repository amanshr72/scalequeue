import { Router } from "express";
import { JobController } from '../controllers/jobController';

const router = Router();
const controller = new JobController();

router.post("/", controller.createJob);
router.get("/:id", controller.getJobById);
router.get("/", controller.listJobs);

export const jobRoutes = router;
