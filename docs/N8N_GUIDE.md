### n8n Integration Guide for GroupScout

This guide provides comprehensive instructions for integrating n8n with GroupScout. It covers triggering the internal pipeline, pushing external leads, and managing automated digests.

---

### 1. Prerequisites

Before you begin, ensure you have the following:

- **GroupScout Server**: Running and accessible (via `go run cmd/server/main.go` or Docker).
- **API Token**: A user-defined secret for authentication.
    - **How to generate**: Run `openssl rand -hex 32` or use the Go tool: `go run -e "import 'crypto/rand'; import 'encoding/hex'; func main() { b := make([]byte, 32); rand.Read(b); println(hex.EncodeToString(b)) }"`
    - **Set it**: In your `.env` file: `API_TOKEN=your_generated_token`.
- **n8n Instance**:
    - **Self-Hosted**: Use the provided `docker-compose.yml`. n8n will be at `http://localhost:5678`.
    - **Cloud/External**: Ensure it can reach your server's IP. If GroupScout is in Docker, use `http://host.docker.internal:8080` (Mac/Win) or the container's IP.

---

### 2. Setting up Authentication in n8n

To simplify your workflows, create a "Header Auth" credential in n8n:

1.  In n8n, go to **Credentials** > **Add Credential**.
2.  Search for **Header Auth**.
3.  **Name**: `GroupScout API`.
4.  **Header Name**: `Authorization`.
5.  **Value**: `Bearer <YOUR_API_TOKEN>` (Replace `<YOUR_API_TOKEN>` with the token from your `.env`).

---

### 3. Triggering the Full Pipeline (`/run`)

Use this to start the GroupScout collection, enrichment, and notification process.

#### Node: **HTTP Request**
- **Method**: `POST`
- **URL**: `http://<groupscout-host>:8080/run`
- **Authentication**: `Predefined Credential` (Select your `GroupScout API` credential).
- **Body Parameters** (Optional):
    - **Mode**: `JSON`
    - **Property**: `bcbid_raw_input`
    - **Value**: `<Raw text for manual BC Bid processing>`

---

### 4. Pushing External Leads (`/n8n/webhook`)

Use this to send leads from other n8n-connected sources (RSS, Web Scrapers, Google Sheets) into GroupScout.

#### Node: **HTTP Request**
- **Method**: `POST`
- **URL**: `http://<groupscout-host>:8080/n8n/webhook`
- **Authentication**: `Predefined Credential`.
- **Body Mode**: `JSON`
- **JSON Payload Example**:
  ```json
  {
    "Title": "New Tech Hub Construction",
    "Location": "Surrey, BC",
    "Source": "Custom RSS Scraper",
    "ProjectValue": 25000000,
    "ProjectType": "Commercial",
    "PriorityScore": 90,
    "PriorityReason": "High value project near transit.",
    "SourceURL": "https://example.com/project",
    "GeneralContractor": "Major Build Inc.",
    "EstimatedCrewSize": 50,
    "OutOfTownCrewLikely": false
  }
  ```

#### Detailed Schema for `/n8n/webhook`:
| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `Title` | String | **Yes** | Name of the project or lead. |
| `Location` | String | No | City, region, or specific address. |
| `Source` | String | No | Source name (defaults to "n8n"). |
| `ProjectValue` | Number | No | Estimated total project value (Integer/Long). |
| `ProjectType` | String | No | e.g., "Commercial", "Hotel", "Residential". |
| `PriorityScore`| Number | No | Relevance score (0-100). |
| `PriorityReason`| String | No | Brief explanation for the score. |
| `SourceURL` | String | No | Direct link to the source document/page. |
| `GeneralContractor`| String | No | Name of the GC. |
| `Applicant` | String | No | Permit applicant name/contact. |
| `Contractor` | String | No | Trade contractor name/contact. |
| `EstimatedCrewSize` | Number | No | Estimated workers needed. |
| `EstimatedDurationMonths` | Number | No | Estimated project length. |
| `OutOfTownCrewLikely` | Boolean | No | `true` if crew likely from outside local area. |
| `SuggestedOutreachTiming` | String | No | e.g., "Immediate", "In 3 months". |
| `Notes` | String | No | Any additional context. |

---

### 5. Automated Weekly Digest (`/digest`)

Trigger a summary email of all "new" or "notified" leads from the last 7 days.

#### Node: **HTTP Request**
- **Method**: `POST`
- **URL**: `http://<groupscout-host>:8080/digest?to=sales@yourcompany.com`
- **Authentication**: `Predefined Credential`.

---

### 6. Scheduling with n8n

You can use the **Schedule** node in n8n to automate GroupScout runs at specific times.

#### Example: Run every Monday and Wednesday at 9:00 AM

1.  Add a **Schedule** node to your workflow.
2.  Set **Trigger Interval** to `Weeks`.
3.  **Days of the Week**: Select `Monday` and `Wednesday`.
4.  **Time**: Set to `09:00`.
5.  Connect this node to an **HTTP Request** node configured for the `/run` endpoint (as described in Section 3).

---

### 7. Troubleshooting & Tips

#### Common Errors
- **401 Unauthorized**: Check your `API_TOKEN` in `.env` matches the `Bearer` token in n8n.
- **400 Bad Request**: Your JSON body is invalid or missing the `Title` field.
- **Connection Refused**: 
    - If n8n is in Docker and GroupScout is on the host, use `http://host.docker.internal:8080`.
    - Check if the GroupScout server is actually running (`go run cmd/server/main.go`).

#### Advanced Workflow Example
1.  **RSS Read**: Check for new construction news.
2.  **AI/LLM (n8n node)**: Summarize the article into structured JSON.
3.  **GroupScout Webhook**: Send the structured JSON to GroupScout.
4.  **Slack (GroupScout Internal)**: GroupScout automatically notifies your team.

---

### 8. Docker Network Note
If you are using the provided `docker-compose.yml`, both services share the same network. You can reach GroupScout from n8n using:
- **URL**: `http://groupscout:8080/run` (or `/n8n/webhook`, `/digest`)
