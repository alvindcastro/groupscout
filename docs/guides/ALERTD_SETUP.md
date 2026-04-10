# alertd — Airport Disruption Alert System

`alertd` is a specialized binary within the GroupScout ecosystem that monitors airport disruptions (weather, cancellations, ground stops) and alerts hotel teams via Slack.

## Architecture

- **Polling Loop:** Runs in the background with two modes:
  - **Quiet Mode (Default):** Polls every 10 minutes.
  - **Active Mode:** Polls every 90 seconds when an alert is active or growing.
- **SPS (Stranded Passenger Score):** A proprietary formula that quantifies the impact on hotel demand.
- **State Machine:** Manages the lifecycle of an event (`Watch → Alert → Updating → Resolved`).
- **Slack Integration:** Uses a Slack Bot token to post and update threaded messages.

## Configuration

Hotel configurations are stored in `config/airports.yaml`.

```yaml
- id: sandman-richmond
  name: Sandman Richmond
  slack_channel: "#alerts-yvr"
  airports:
    - code: CYVR
      distance_min: 8
  alert_threshold_sps: 60
  distressed_rate: 119
  rack_rate: 220
  airline_contacts:
    - carrier: Air Canada
      ops_phone: "604-276-7477"
```

### Environment Variables

- `SLACK_BOT_TOKEN`: (Required) The Bot User OAuth Token for the Slack app.
- `ALERTD_CONFIG_PATH`: (Optional) Custom path to the YAML config (defaults to `config/airports.yaml`).

## Deployment

`alertd` runs as a separate service in `docker-compose.yml`:

```bash
docker-compose up -d alertd
```

## Monitoring & Logs

`alertd` logs to stdout in the same format as the main GroupScout server. It includes exponential backoff logs if external APIs (ECCC, YVR, NavCanada) fail.
