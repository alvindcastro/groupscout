### API Testing Guide

This guide provides instructions on how to test GroupScout's internal API endpoints using various tools.

---

### 1. Prerequisite: API Token
Most POST endpoints require a Bearer Token for authentication. This token is defined by the `API_TOKEN` environment variable in your `.env` file.

If `API_TOKEN` is not set, the server will allow all requests (insecure mode, intended for local development only).

---

### 2. Testing with `curl`

#### Lead Generation Server (Port 8080)

**Health Check**
```bash
curl -i http://localhost:8080/health
```

**Manual Pipeline Run**
```bash
curl -i -X POST \
  -H "Authorization: Bearer your_api_token" \
  -H "Content-Type: application/json" \
  -d '{"bcbid_raw_input": ""}' \
  http://localhost:8080/run
```

**Trigger Weekly Digest**
```bash
curl -i -X POST \
  -H "Authorization: Bearer your_api_token" \
  "http://localhost:8080/digest?to=alvin@groupscout.ai"
```

**n8n Webhook Simulation**
```bash
curl -i -X POST \
  -H "Authorization: Bearer your_api_token" \
  -H "Content-Type: application/json" \
  -d '{
    "Source": "curl_test",
    "Title": "Simulated Lead",
    "Location": "Vancouver, BC",
    "ProjectValue": 500000
  }' \
  http://localhost:8080/n8n/webhook
```

#### Alerting Service (`alertd`) (Port 8081)

**Slack Inventory Update**
```bash
curl -i -X POST \
  -d "text=50" \
  http://localhost:8081/slack/inventory
```

---

### 3. Testing with Bruno (Recommended)

[Bruno](https://www.usebruno.com/) is a fast, open-source API client. A collection for GroupScout is included in the repository.

1.  **Open Bruno**.
2.  **Open Collection**: Point it to the `api/bruno` folder in this repository.
3.  **Select Environment**: Click the top-right environment selector and choose **Local**.
4.  **Configure Token**: Edit the **Local** environment variables to match your `API_TOKEN`.
5.  **Run Requests**: You can now run all pre-configured requests.

---

### 4. Testing with OpenAPI / Swagger

An OpenAPI 3.0 specification is available at `api/swagger.yaml`.

-   **Visualizing**: You can paste the content of `api/swagger.yaml` into the [Swagger Editor](https://editor.swagger.io/) or use a local Swagger UI instance.
-   **Servers**: The spec defines two servers:
    -   `http://localhost:8080` (Lead Generation)
    -   `http://localhost:8081` (Alerting Service)

---

### 5. Troubleshooting

-   **401 Unauthorized**: Ensure your `Authorization` header matches the `API_TOKEN` set in the `.env` file.
-   **404 Not Found**: Verify you are using the correct port for the service (8080 for Leads, 8081 for Alerts).
-   **503 Service Unavailable**: Usually means the database connection is failing. Check your `DATABASE_URL`.
-   **Connection Refused**: Ensure the binary is running (`go run cmd/server/main.go` or `go run cmd/alertd/main.go`).
