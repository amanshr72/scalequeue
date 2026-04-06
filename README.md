# ScaleQueue

Distributed job queue system built with Node.js, Redis, and Go — designed to handle high-throughput async processing with reliability guarantees like retries, prioritization, and dead-letter handling.

## Overview

Most backend systems break at async processing — retries cause duplicate execution, failures lead to lost jobs, and systems struggle under load.

ScaleQueue solves this with:
- Reliable background job processing
- Retry-safe (idempotent) execution
- Scalable worker architecture

## Architecture

Client → API Service (Node.js) → Redis Queue → Worker Service (Go) → Processing  
                                                     ↓  
                                                Dashboard

## Components

**API Service (Node.js)**
- Job enqueue APIs
- Priority handling
- Retry configuration
- Job status tracking

**Worker Service (Go)**
- Concurrent job execution
- Retry with backoff
- Dead-letter queue handling
- Fault-tolerant processing

**Redis**
- Queue storage
- State management
- Job coordination

**Dashboard**
- Job monitoring (pending, processing, failed)
- Queue visibility

## Features

- Priority-based job scheduling  
- Retry mechanism with backoff  
- Dead-letter queue (DLQ)  
- Concurrent worker processing  
- Real-time job tracking  
- Horizontally scalable workers  

## Key Engineering Decisions

**Idempotency**  
Jobs are retry-safe to prevent duplicate execution.

**Fault Tolerance**  
Failures are handled via retries and DLQ — no job loss.

**Separation of Concerns**  
API handles ingestion, workers handle execution, Redis handles coordination.

**Tech Stack Split**  
Node.js for API (I/O heavy) and Go for workers (concurrency + performance).

## Getting Started

### Prerequisites
- Node.js
- Go
- Redis

### Start Redis
redis-server

### Run API Service
cd api-service  
npm install  
npm run dev  

### Run Worker Service
cd worker-service  
go run cmd/main.go  

### Run Dashboard
cd dashboard  
node app.js  

## Example Flow

1. Client sends job request to API  
2. API pushes job to Redis  
3. Worker processes job  
4. Failures → retry or DLQ  
5. Dashboard reflects status  

## Use Cases

- Payment processing (retry-safe workflows)  
- Email/SMS queues  
- Background jobs  
- Event-driven systems  

## Future Improvements

- Rate limiting per queue  
- Job scheduling (cron)  
- Observability (metrics, tracing)  
- Multi-tenant isolation  

## Author

Aman Sharma  
Backend Engineer — Node.js, Golang, Distributed Systems
