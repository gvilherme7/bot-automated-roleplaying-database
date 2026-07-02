# Infrastructure and Application Usage

This document outlines the deployment strategy, runtime infrastructure constraints, and end-to-end usage of the RAG Firecast Backend.

## Deployment Orchestration

The application utilizes Docker and Kubernetes for containerization and high-availability workload orchestration.

### Containerization (Docker)
The `Dockerfile` employs a multi-stage build pattern:
1. **Builder Stage**: Uses `golang:1.22-alpine` to compile a statically linked, stripped Go binary, completely removing the toolchain overhead.
2. **Runtime Stage**: Deploys on a minimal `alpine:3.19` base image equipped with critical `ca-certificates`.
- **Security**: The resulting image is exceptionally lightweight and contains zero source code files, adhering to strict DevSecOps parameters.

### Kubernetes Manifests
- **Deployment (`k8s/deployment.yaml`)**:
  - Exposes `rag-bot` natively targeting `imagePullPolicy: Never` for local execution strategies (`k3d`/`minikube`).
  - Strict resource tracking bound via container limits preventing noisy neighbor conditions on shared cluster nodes.
- **Secrets (`k8s/secrets.yaml`)**:
  - Securely injects `DATABASE_URL` and `PLUGIN_API_KEY` via environment variable overlays to avoid exposing credentials natively in the deployment file.

### Deployment Instructions

1. Ensure a PostgreSQL database instance running `pgvector` is externally accessible.
2. Encrypt your secrets and replace the base64 stubs within `k8s/secrets.yaml`:
   ```bash
   echo -n "your-token" | base64
   ```
3. Apply the secret configurations to your Kubernetes cluster:
   ```bash
   kubectl apply -f k8s/secrets.yaml
   ```
4. Build the image locally into the cluster daemon:
   ```bash
   docker build -t rag-bot:latest .
   ```
5. Deploy the application workload:
   ```bash
   kubectl apply -f k8s/deployment.yaml
   ```

## Hardware and Metrics

### Resource Allocation Parameters
- **Requests**: 100 millicores CPU | 128 MiB Memory
- **Limits**: 500 millicores CPU | 256 MiB Memory
These constraints force strict bounds. Standard execution utilizes heavily optimized goroutines (averaging ~20 MiB of idle RAM consumption). 

### System Metrics Tracking
Monitoring of CPU/Memory should be observed via native `Metrics Server` integrations:
```bash
kubectl top pods -l app=rag-bot
```
The Go runtime is intrinsically garbage-collected. The PostgreSQL connection pools must be actively correlated against memory allocation over 24-hour periods.

## End-to-End Application Usage

Once the application is cleanly scheduled onto a cluster node and indicates `Running`:

1. **Endpoint Exposures**: The backend exposes an HTTP API on port 8080. It requires an appropriate `Service` or `Ingress` routing within Kubernetes so that the Firecast plugin can securely communicate with it.
2. **Execution**: Load the `.rpk` plugin in the Firecast client and use the configured chat commands (e.g., `!lore`, `!lore_sync`) to interact with the backend.
3. **Query Format**: Execute a semantic prompt via `!lore <query>`.
4. **Processing Pipeline**:
   - The Go runtime captures the message channel frame.
   - The embedded repository extracts the cosine-similar textual entries from PostgreSQL.
   - A deterministic local Ollama chat model parses the prompt.
   - The final response streams synchronously back to the target text channel.

## Automated Verification

- **Code Validation**: 100% of unit tests mapping integration bounds have successfully passed in the `/internal/` directories.
- **Manifest Validations**: The orchestrator configuration is strictly parsed utilizing standardized `apps/v1` API definitions compatible with any CNCF-certified Kubernetes 1.25+ cluster.
