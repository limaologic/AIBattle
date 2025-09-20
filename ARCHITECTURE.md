# Reverse Challenge System - Technical Architecture

## 🎯 System Overview

The Reverse Challenge System inverts the traditional challenge-solving paradigm. Instead of solvers submitting code to a central platform, **Challengers proactively push problems to Solvers**, who process them using their own computational resources and return results via authenticated callbacks.

## 🏗️ High-Level Architecture

```
┌─────────────────────┐                    ┌─────────────────────┐
│    CHALLENGER       │                    │       SOLVER        │
│                     │                    │                     │
│ ┌─────────────────┐ │   ①  Push Challenge │ ┌─────────────────┐ │
│ │ Challenge Store │ │──────────────────────→│ Receive Handler │ │
│ └─────────────────┘ │   (HMAC Signed)    │ └─────────────────┘ │
│                     │                    │          │          │
│ ┌─────────────────┐ │                    │          ▼          │
│ │ Callback Handler│ │   ②  Async Callback │ ┌─────────────────┐ │
│ └─────────────────┘ │◄─────────────────────│ Worker Pool     │ │
│          │          │   (HMAC Signed)    │ │• Process Tasks  │ │
│          ▼          │                    │ │• Retry Logic    │ │
│ ┌─────────────────┐ │                    │ │• Backoff/Jitter │ │
│ │ Answer Validator│ │                    │ └─────────────────┘ │
│ │• Exact Match    │ │                    │                     │
│ │• Numeric Range  │ │                    │ ┌─────────────────┐ │
│ │• Regex Pattern  │ │                    │ │    Mock Solver  │ │
│ └─────────────────┘ │                    │ │• CAPTCHA OCR    │ │
│          │          │                    │ │• Math Compute   │ │
│          ▼          │                    │ │• Text Process   │ │
│ ┌─────────────────┐ │                    │ └─────────────────┘ │
│ │  Results Store  │ │                    │                     │
│ └─────────────────┘ │                    └─────────────────────┘
│                     │
│ ┌─────────────────┐ │
│ │   SQLite DB     │ │      🔒 Security Features:
│ │• challenges     │ │      • HMAC-SHA256 Authentication  
│ │• results        │ │      • Nonce-based Replay Protection
│ │• webhooks       │ │      • Time-window Validation
│ │• seen_nonces    │ │      • HTTPS-only Callbacks
│ └─────────────────┘ │      • Request Size Limiting
└─────────────────────┘
```

## 🔐 Security Architecture

### HMAC Authentication Flow

```
1. Request Preparation:
   ┌─────────────────┐
   │ Original Request│
   │ (Method + Path  │
   │  + Headers      │
   │  + Body)        │
   └─────────┬───────┘
             │
             ▼
   ┌─────────────────┐
   │ Canonical String│──────┐
   │ METHOD\n        │      │
   │ PATH\n          │      │  
   │ TIMESTAMP\n     │      │
   │ NONCE\n         │      ▼
   │ SHA256(body)    │   ┌──────────┐
   └─────────────────┘   │HMAC-SHA256│
                         │(secret,   │
                         │canonical) │
   ┌─────────────────┐   └────┬─────┘
   │ Authorization:  │        │
   │ RCS-HMAC-SHA256 │◄───────┘
   │ keyId=xxx,      │
   │ ts=xxx,         │
   │ nonce=xxx,      │
   │ sig=xxx         │
   └─────────────────┘

2. Verification Process:
   Request────┐
             ▼
   ┌─────────────────┐
   │Parse Auth Header│
   └─────────┬───────┘
             ▼
   ┌─────────────────┐     ❌ Reject
   │Check Time Window│────────────→
   │(±300 seconds)   │
   └─────────┬───────┘
             ▼
   ┌─────────────────┐     ❌ Reject
   │Check Nonce Seen │────────────→
   │(Replay Attack)  │
   └─────────┬───────┘
             ▼
   ┌─────────────────┐     ❌ Reject  
   │Verify Signature │────────────→
   │hmac.Equal()     │
   └─────────┬───────┘
             ▼
           ✅ Accept
```

### Nonce Management

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Generate UUID   │    │ Check Database  │    │  Store Nonce    │
│ for Request     │───→│ for Duplicate   │───→│  with Timestamp │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                       │
                                ▼                       ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │ Reject if Found │    │ Periodic Cleanup│
                       │ (Replay Attack) │    │ (Hourly Process)│
                       └─────────────────┘    └─────────────────┘
```

## 🔄 Challenge Processing Flow

### 1. Challenge Creation & Distribution

```
Challenger Process:
┌─────────────────┐
│ Create Challenge│
│ • Problem data  │
│ • Output spec   │  
│ • Validation    │
│ • Answer (local)│
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Store in DB     │
│ (with answer)   │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Send to Solver  │
│ • NO answer     │
│ • NO validation │
│ • Only problem  │
│ • Only output   │
│   specification │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ HMAC Sign &     │
│ HTTP POST       │
│ /solve endpoint │
└─────────────────┘
```

### 2. Solver Processing Pipeline

```
Solver Process:
┌─────────────────┐
│ Receive Challenge│
│ via /solve      │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Validate Request│
│ • HMAC signature│
│ • Content format│
│ • Required fields│
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Queue in DB     │
│ Status: pending │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Worker Pool     │
│ Picks up task   │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Process Problem │
│ • CAPTCHA → OCR │
│ • Math → Calc   │
│ • Text → Transform│
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Send Callback   │
│ with Answer     │
└─────────────────┘
```

### 3. Worker Pool Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Worker Pool Manager                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    Job Queue    ┌─────────────┐           │
│  │ Dispatcher  │───────────────►│             │           │
│  │ (DB Poller) │   (Buffered     │             │           │
│  │ Every 5s    │    Channel)     │             │           │
│  └─────────────┘                 │             │           │
│                                   │             │           │
│  ┌─────────────────────────────────┴──┐          │           │
│  │            Job Queue               │          │           │
│  │ ┌───┐┌───┐┌───┐┌───┐┌───┐┌───┐   │          │           │
│  │ │ P ││ P ││ P ││ P ││ P ││ P │... │          │           │
│  │ └───┘└───┘└───┘└───┘└───┘└───┘   │          │           │
│  └─────────────────────┬──────────────┘          │           │
│                        │                         │           │
│         ┌──────────────┼──────────────┐          │           │
│         ▼              ▼              ▼          ▼           │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐ │
│  │ Worker 1  │  │ Worker 2  │  │ Worker 3  │  │ Worker N  │ │
│  │           │  │           │  │           │  │           │ │
│  │ Process   │  │ Process   │  │ Process   │  │ Process   │ │
│  │ Challenge │  │ Challenge │  │ Challenge │  │ Challenge │ │
│  │           │  │           │  │           │  │           │ │
│  │ Send      │  │ Send      │  │ Send      │  │ Send      │ │
│  │ Callback  │  │ Callback  │  │ Callback  │  │ Callback  │ │
│  │ w/ Retry  │  │ w/ Retry  │  │ w/ Retry  │  │ w/ Retry  │ │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 4. Retry Logic with Exponential Backoff

```
Retry Sequence:
Attempt 1: Immediate
     │
     ├─ Success ───► End
     │
     ├─ 4xx Error (not 429) ───► Fail (No Retry)
     │
     └─ 429/5xx/Network Error
             │
             ▼
Attempt 2: Wait 500ms * jitter
     │
     ├─ Success ───► End  
     │
     └─ Retryable Error
             │
             ▼
Attempt 3: Wait 1000ms * jitter
     │
     └─ Continue...
             │
             ▼
Max Attempts: 6
Final Failure: Mark task as failed

Backoff Calculation:
delay = min(30s, 500ms * 2^(attempt-1)) * jitter
where jitter = random(0.85, 1.15)
```

## 💾 Data Architecture

### Database Schema Design

```
CHALLENGER DATABASE:
┌──────────────────────────────────────────────────────────┐
│ challenges                                               │
├──────────────────────────────────────────────────────────┤
│ id (TEXT, PRIMARY KEY)                                   │
│ type (TEXT) ──────────────────► "captcha", "math", etc  │
│ problem (TEXT) ───────────────► JSON blob sent to solver│
│ output_spec (TEXT) ───────────► Expected output format  │
│ validation_rule (TEXT) ────────► JSON with answer ⚠️    │
│ created_at (TIMESTAMP)                                   │
└──────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────┐
│ results                                                  │
├──────────────────────────────────────────────────────────┤
│ id (INTEGER, AUTOINCREMENT)                              │
│ challenge_id (TEXT, FK)                                  │
│ request_id (TEXT) ─────────────► X-Request-ID (idempotent)│
│ solver_job_id (TEXT)                                     │
│ status (TEXT) ─────────────────► "success", "failed"    │
│ received_answer (TEXT)                                   │
│ is_correct (BOOLEAN) ──────────► Validation result       │
│ compute_time_ms (INTEGER)                                │
│ solver_metadata (TEXT) ────────► JSON from solver        │
│ created_at (TIMESTAMP)                                   │
│ UNIQUE(challenge_id, request_id)                         │
└──────────────────────────────────────────────────────────┘

SOLVER DATABASE:
┌──────────────────────────────────────────────────────────┐
│ pending_challenges                                       │
├──────────────────────────────────────────────────────────┤
│ id (TEXT, PRIMARY KEY)                                   │
│ problem (TEXT) ────────────────► JSON problem data       │
│ output_spec (TEXT) ────────────► Expected format         │
│ callback_url (TEXT) ───────────► Where to send result    │
│ received_at (TIMESTAMP)                                  │
│ status (TEXT) ─────────────────► "pending", "processing", │
│                                  "completed", "failed"   │
│ attempt_count (INTEGER)                                  │
│ next_retry_time (TIMESTAMP)                              │
└──────────────────────────────────────────────────────────┘
```

### Answer Security Model

```
CRITICAL SECURITY PRINCIPLE:

┌─────────────────┐              ┌─────────────────┐
│   CHALLENGER    │              │     SOLVER      │
│                 │              │                 │
│ ✅ Has Answer   │              │ ❌ No Answer    │
│ ✅ Has Validation│              │ ❌ No Validation │
│ ✅ Verifies     │              │ ❌ Cannot Verify │
│ ✅ Stores Result│              │ ❌ No Results   │
│                 │              │                 │
│ NEVER SENDS:    │   Network    │ ONLY RECEIVES:  │
│ • Answer        │◄────────────►│ • Problem Data  │
│ • Validation    │   Traffic    │ • Output Spec   │
│ • Success/Fail  │              │ • Constraints   │
└─────────────────┘              └─────────────────┘

Data Flow Validation:
1. Challenger creates problem + answer
2. Challenger stores answer locally only
3. Challenger sends problem (NO ANSWER) to solver
4. Solver processes and returns candidate answer
5. Challenger validates answer against local truth
6. Challenger stores validation result locally
```

## 🚀 Performance Characteristics

### Throughput & Latency

```
Expected Performance (Single Instance):
┌─────────────────────────────────────────┐
│ Component        │ Metric              │
├─────────────────────────────────────────┤
│ Challenger       │ 100 challenges/sec  │
│ Callback Handler │ 200 callbacks/sec   │
│ Solver Workers   │ 50 tasks/sec        │
│ HMAC Verification│ 1000 requests/sec   │
│ SQLite (WAL)     │ 2000 writes/sec     │
└─────────────────────────────────────────┘

Latency Distribution:
┌─────────────────────────────────────────┐
│ Operation        │ p50    │ p95   │p99  │
├─────────────────────────────────────────┤
│ Challenge Send   │ 50ms   │ 200ms │400ms│
│ Callback Receive │ 10ms   │ 50ms  │100ms│
│ Answer Validation│ 1ms    │ 5ms   │10ms │
│ Database Write   │ 5ms    │ 20ms  │50ms │
└─────────────────────────────────────────┘
```

### Scaling Limits

```
Single Instance Limits:
• Challenger: ~10,000 active challenges
• Solver: ~1,000 pending tasks  
• Database: ~100MB (1M challenges)
• Memory: ~50MB (Go runtime + SQLite cache)

Scale-Out Options:
┌─────────────────────────────────────────┐
│ Component     │ Scaling Strategy        │
├─────────────────────────────────────────┤
│ Challenger    │ Load balancer + session │
│               │ sticky for callbacks    │
│ Solver        │ Independent instances   │
│ Database      │ PostgreSQL + replicas   │
│ Nonce Store   │ Redis cluster          │
└─────────────────────────────────────────┘
```

## 🔧 Operational Architecture

### Health Check Strategy

```
Health Check Hierarchy:
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   /healthz      │    │    /readyz      │    │     /stats      │
│                 │    │                 │    │                 │
│ • Process alive │    │ • DB connection │    │ • Queue depth   │
│ • Port binding  │    │ • Disk space    │    │ • Success rates │
│ • Basic routing │    │ • Memory usage  │    │ • Retry counts  │
│                 │    │ • Dependency    │    │ • Latency dist  │
│                 │    │   services      │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
      Fast                   Thorough              Diagnostic
    (~1ms)                  (~50ms)               (~100ms)
```

### Observability Stack

```
Logging Pipeline:
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Structured JSON │    │ Request Tracing │    │  Aggregation    │
│ (zerolog)       │───→│ • request_id    │───→│ • ELK Stack     │
│                 │    │ • challenge_id  │    │ • Grafana       │
│ • timestamp     │    │ • user_agent    │    │ • Alerts        │
│ • level         │    │ • duration      │    │                 │
│ • message       │    │ • status_code   │    │ Query Examples: │
│ • fields        │    │ • error_code    │    │ • Error rates   │
└─────────────────┘    └─────────────────┘    │ • Slow requests │
                                              │ • Auth failures │
Metrics Collection:                           └─────────────────┘
┌─────────────────┐
│ Application     │
│ • Request count │
│ • Response time │
│ • Error rate    │
│ • Queue depth   │
│ • Worker util   │
└─────────────────┘
```

### Deployment Patterns

```
Development:
┌────────────┐  ┌────────────┐  ┌────────────┐
│   ngrok    │  │ Challenger │  │   Solver   │
│ (public)   │  │ :8080      │  │ :8081      │
│            │  │            │  │            │
│ Internet───┼──┼─ localhost ─┼──┼─localhost │
│            │  │            │  │            │
└────────────┘  └────────────┘  └────────────┘

Production:
┌────────────┐  ┌────────────┐  ┌────────────┐
│ Load       │  │ Challenger │  │   Solver   │
│ Balancer   │  │ Cluster    │  │ Cluster    │
│ (nginx)    │  │            │  │            │
│            │  │ ┌────────┐ │  │ ┌────────┐ │
│ Internet───┼──┼─│Instance│ │  │ │Instance│ │
│            │  │ └────────┘ │  │ └────────┘ │
│            │  │ ┌────────┐ │  │ ┌────────┐ │
│            │  │ │Instance│ │  │ │Instance│ │
│            │  │ └────────┘ │  │ └────────┘ │
└────────────┘  └────────────┘  └────────────┘
                      │              │
              ┌───────┼──────────────┼───────┐
              │   Shared Infrastructure     │
              │ ┌──────────┐ ┌──────────┐   │
              │ │PostgreSQL│ │  Redis   │   │  
              │ │(Primary) │ │ (Nonces) │   │
              │ └──────────┘ └──────────┘   │
              └─────────────────────────────┘
```

This architecture provides a robust, scalable, and secure foundation for the Reverse Challenge System, with clear separation of concerns and comprehensive security measures.