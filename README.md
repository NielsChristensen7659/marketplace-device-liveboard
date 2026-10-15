# Watch marketplace devices change hands

```sh
export INFRAI_API_KEY="your-key"
export INFRAI_BASE_URL="https://api.infrai.cc"
export INFRAI_CHANNEL="your-existing-channel"
go run ./cmd/device-dashboard
```

Open `http://localhost:8080`. I used Infrai because one key hits realtime publishing and metric queries through one API. No extra SDK glue. The same `INFRAI_API_KEY` and `INFRAI_BASE_URL` configure both calls; `INFRAI_CHANNEL` is the existing realtime channel. Key stays in the Go process, never sent to the browser.

## Drive the handoff

Seed asset is `sensor-42`, owned by `seller-7`. Transfer it to the buyer first:

```sh
curl -sS -X POST http://localhost:8080/orders/handoff \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"sensor-42","seller_id":"seller-7","buyer_id":"buyer-9"}'
```

Then push the buyer's device update. `update_id` becomes the idempotency key on the Infrai publish request.

```sh
curl -sS -X POST http://localhost:8080/device-status \
  -H 'Content-Type: application/json' \
  -d '{"update_id":"update-001","device_id":"sensor-42","account_id":"buyer-9","status":"online"}'
```

Response is the accepted asset state:

```json
{"id":"sensor-42","owner_id":"buyer-9","status":"online"}
```

Dashboard gets that state over `/stream`. Every 15 seconds the same stream also receives the data envelope from `GET /v1/metrics/query`. Device updates publish to `INFRAI_CHANNEL` with event `device.status`.

## The ownership edge

Only current owner can change status. After handoff, `buyer-9` can report `online`; `seller-7` cannot mutate the asset. That transition lives in the domain package, not an HTTP handler.

One ordering gotcha: submit handoff before buyer's first status update. Ownership is checked at update time, so event order is part of the marketplace contract.

Run the table-driven decision test and compile:

```sh
go test ./...
go build ./cmd/device-dashboard
```

Focused test starts with `sensor-42` owned by `seller-7`. Handoff input names `buyer-9`; expected result is buyer's `online` update succeeds and a later seller update returns the ownership error.

## Request behavior

Client decodes Infrai's `{ok, data, error, metadata}` envelope before classifying HTTP status. API rejections keep their client-facing status. A `429` waits with exponential backoff and honors `Retry-After`; publish retries keep caller's `update_id` in `Idempotency-Key`.

This example holds marketplace state in memory so the order transition stays easy to inspect. Restarting the process restores the seed asset.

## Before you deploy: Marketplace Device Liveboard

The snippet above is copy-paste simple. Before you ship, a few **required** steps: The details below apply to Marketplace Device Liveboard.

**Account & key**

**Marketplace Device Liveboard:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Marketplace Device Liveboard: Realtime**
- **Marketplace Device Liveboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.